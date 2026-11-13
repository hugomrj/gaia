// internal/handlers/api.go
package handlers

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"

	"gaia/internal/brain"
	"gaia/internal/models"
	"gaia/internal/providers"
	"gaia/internal/providers/mistral"
	"gaia/internal/tenant"

	"github.com/gin-gonic/gin"
)

type APIHandler struct {
	brain *brain.Brain
	store *tenant.Store
}

func NewAPIHandler(b *brain.Brain, store *tenant.Store) *APIHandler {
	return &APIHandler{brain: b, store: store}
}

// ============================================================
// ANALYZE
// ============================================================
func (h *APIHandler) Analyze(c *gin.Context) {
	t := c.MustGet("tenant").(*tenant.Tenant)

	var req models.ChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, models.ErrInvalidRequest(err.Error()))
		return
	}

	llm, apiKey, err := h.resolveLLM(t, c)
	if err != nil {
		writeError(c, err)
		return
	}

	analysis, err := h.brain.ProcessMessage(c.Request.Context(), req, llm)
	if err != nil {
		writeError(c, err)
		return
	}

	h.bumpUsage(apiKey.ID)
	c.JSON(http.StatusOK, analysis)
}

// ============================================================
// SYNTHESIZE
// ============================================================
func (h *APIHandler) Synthesize(c *gin.Context) {
	t := c.MustGet("tenant").(*tenant.Tenant)

	var req models.SynthesisRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, models.ErrInvalidRequest(err.Error()))
		return
	}

	llm, apiKey, err := h.resolveLLM(t, c)
	if err != nil {
		writeError(c, err)
		return
	}

	resp, err := h.brain.ProcessResult(c.Request.Context(), req, llm)
	if err != nil {
		writeError(c, err)
		return
	}

	h.bumpUsage(apiKey.ID)
	c.JSON(http.StatusOK, resp)
}

// ============================================================
// NORMALIZE  (determinístico, no toca LLM ni X-Provider)
// ============================================================
func (h *APIHandler) Normalize(c *gin.Context) {
	_ = c.MustGet("tenant") // valida que el middleware lo inyectó

	var req models.NormalizeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, models.ErrInvalidRequest(err.Error()))
		return
	}

	resp := h.brain.Normalize(req.Parameters)
	c.JSON(http.StatusOK, resp)
}

// ============================================================
// RESOLUCIÓN DE PROVIDER + KEY  (BYOK)
// ============================================================
func (h *APIHandler) resolveLLM(t *tenant.Tenant, c *gin.Context) (providers.LLMProvider, *tenant.APIKey, error) {
	// 1. Recolectar providers con al menos una key activa
	active := []string{}
	for _, p := range t.Providers {
		for _, k := range p.Keys {
			if k.IsActive && k.APIKeyPlain != "" {
				active = append(active, p.ProviderName)
				break
			}
		}
	}

	if len(active) == 0 {
		return nil, nil, models.ErrInvalidRequest(
			"el tenant no tiene ningún proveedor con API key activa configurada")
	}

	// 2. Determinar el provider seleccionado
	headerProvider := strings.ToLower(strings.TrimSpace(c.GetHeader("X-Provider")))
	headerModel := strings.TrimSpace(c.GetHeader("X-Model")) // <-- NUEVO: Leer el modelo

	var selected string
	if headerProvider != "" {
		found := false
		for _, p := range active {
			if p == headerProvider {
				found = true
				break
			}
		}
		if !found {
			// Distinguir "vinculado pero sin key" vs "no vinculado"
			for _, p := range t.Providers {
				if p.ProviderName == headerProvider {
					return nil, nil, models.ErrInvalidRequest(
						fmt.Sprintf("el proveedor %q está vinculado al tenant pero no tiene API key activa", headerProvider))
				}
			}
			return nil, nil, models.ErrInvalidRequest(
				fmt.Sprintf("el tenant no tiene el proveedor %q configurado. Disponibles: %s",
					headerProvider, strings.Join(active, ", ")))
		}
		selected = headerProvider
	} else {
		if len(active) > 1 {
			return nil, nil, models.ErrInvalidRequest(
				fmt.Sprintf("el tenant tiene múltiples proveedores activos; especificá el header X-Provider. Disponibles: %s",
					strings.Join(active, ", ")))
		}
		selected = active[0]
	}

	// 3. Obtener una key activa al azar entre las del tenant+provider
	key, ok := pickRandomActiveKey(t, selected)
	if !ok {
		return nil, nil, models.ErrInvalidRequest(
			fmt.Sprintf("no se encontró API key activa para el proveedor %q", selected))
	}

	// 4. Construir el cliente LLM (pasando el modelo)
	llm, err := buildProvider(selected, key.APIKeyPlain, headerModel)
	if err != nil {
		return nil, nil, err
	}

	// Agregamos el modelo al log para saber qué se usó
	log.Printf("[API] tenant=%s provider=%s key_id=%d model=%s", t.Slug, selected, key.ID, headerModel)
	return llm, key, nil
}

// pickRandomActiveKey busca el TenantProvider por nombre y delega
// la elección aleatoria entre sus keys activas.
func pickRandomActiveKey(t *tenant.Tenant, providerName string) (*tenant.APIKey, bool) {
	for i := range t.Providers {
		if t.Providers[i].ProviderName == providerName {
			return t.Providers[i].PickRandomActiveKey()
		}
	}
	return nil, false
}

// buildProvider instancia el cliente LLM correspondiente.
func buildProvider(name, apiKey, model string) (providers.LLMProvider, error) {
	switch name {
	case "mistral":
		// El cliente de Mistral ya maneja el default si model == ""
		return mistral.NewClient(apiKey, model), nil
	// case "openai":
	//     return openai.NewClient(apiKey, model), nil
	default:
		return nil, models.ErrInvalidRequest(
			fmt.Sprintf("proveedor %q no soportado por esta versión de GAIA", name))
	}
}

// bumpUsage incrementa usage_count en background.
func (h *APIHandler) bumpUsage(keyID int64) {
	go func() {
		if err := h.store.IncrementUsageCount(keyID); err != nil {
			log.Printf("[API] no se pudo incrementar usage_count (key_id=%d): %v", keyID, err)
		}
	}()
}

// ============================================================
// writeError — sin cambios
// ============================================================
func writeError(c *gin.Context, err error) {
	var gErr *models.GAIAError
	if !errors.As(err, &gErr) {
		log.Printf("[API] error no estructurado: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "internal_error", "message": "error interno", "retry": false,
		})
		return
	}

	status := http.StatusInternalServerError
	switch gErr.Code {
	case "rate_limit":
		status = http.StatusTooManyRequests
	case "timeout":
		status = http.StatusGatewayTimeout
	case "auth":
		status = http.StatusBadGateway
	case "upstream":
		status = http.StatusServiceUnavailable
	case "invalid_request":
		status = http.StatusBadRequest
	case "internal_error":
		status = http.StatusInternalServerError
	}

	log.Printf("[API] %s: %s (retry=%v)", gErr.Code, gErr.Message, gErr.Retry)
	c.JSON(status, gin.H{
		"error": gErr.Code, "message": gErr.Message, "retry": gErr.Retry,
	})
}