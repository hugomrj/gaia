# GAIA

Orquestador conversacional para sistemas empresariales, escrito en Go.

GAIA es un motor de razonamiento que permite que un usuario hable en
lenguaje natural y que un sistema empresarial (ERP, CRM, bot, etc.) entienda
qué hacer. GAIA no toca datos sensibles ni ejecuta funciones: solo analiza,
normaliza y redacta. El cliente es el que actúa.

## Filosofía

"GAIA piensa, el cliente actúa."

- GAIA es stateless: no guarda sesiones ni estado.
- GAIA no conoce las funciones del cliente de antemano.
- GAIA no ejecuta código ni toca bases de datos.
- El cliente es el agente: orquesta, ejecuta y mantiene el historial.

## Arquitectura

Usuario → Cliente → POST /v1/analyze   → Planner      (¿qué quiere?)
                  → POST /v1/normalize → Normalizer   (opcional, limpia datos)
                  → (cliente ejecuta la función real)
                  → POST /v1/synthesize→ Synthesizer  (¿cómo lo digo?)
                  → Usuario

### Componentes

Planner      → Entiende el lenguaje natural. Devuelve intent + función + parámetros.
Normalizer   → Convierte valores crudos ("hoy", "150 mil") a formato estándar.
Synthesizer  → Redacta la respuesta final al usuario en español.
Brain        → Orquesta los 3 componentes.

## Requisitos

- Go 1.25+
- API key de Mistral — https://console.mistral.ai
- Git

## Setup

### 1. Clonar el proyecto

    git clone <repo-url> gaia
    cd gaia

### 2. Instalar dependencias

    go mod download

### 3. Crear el archivo .env

En la raíz del proyecto, crear .env con:

    MISTRAL_API_KEY=tu_api_key_aqui
    GAIA_ADMIN_USER=admin
    GAIA_ADMIN_PASSWORD_HASH=
    GAIA_TIMEZONE=America/Asuncion

### 4. Generar el hash de la contraseña del admin

    go run ./cmd/hash

Te va a pedir una contraseña y devuelve el hash. Pegalo en
GAIA_ADMIN_PASSWORD_HASH del .env.

Ejemplo del .env final:

    MISTRAL_API_KEY=sk-xxxxxxxxxxxxxxxxxxxx
    GAIA_ADMIN_USER=admin
    GAIA_ADMIN_PASSWORD_HASH=$2a$10$abcdef...
    GAIA_TIMEZONE=America/Asuncion

IMPORTANTE: El archivo .env NUNCA se sube a Git. Verificá que esté en
.gitignore.

## Ejecución

### Modo desarrollo

    go run ./cmd/app

GAIA estará disponible en http://localhost:8080.

### Modo compilado

    go build -o bin/gaia ./cmd/app
    ./bin/gaia

## API

### GET /health

Health check.

Respuesta:

    {"name":"GAIA","status":"running","version":"0.1.0"}

### POST /v1/analyze

El Planner analiza el mensaje del usuario y devuelve intent + función +
parámetros.

Request:

    {
      "session_id": "abc-123",
      "message": "cuanto facture hoy",
      "collected_data": null,
      "history": []
    }

Response:

    {
      "intent": "query_billing",
      "function_name": "get_daily_billing",
      "parameters": {"date": "hoy"},
      "confidence": 0.95,
      "original_input": "cuanto facture hoy"
    }

### POST /v1/normalize

Convierte valores crudos a formato estándar (opcional).

Request:

    {
      "parameters": {
        "date": "el lunes",
        "amount": "150 mil"
      }
    }

Response:

    {
      "parameters": {
        "date": "2026-10-19",
        "amount": 150000
      },
      "normalized": ["date", "amount"],
      "not_normalized": []
    }

### POST /v1/synthesize

El Synthesizer redacta la respuesta al usuario. Tiene 4 modos automáticos:
conversation, clarify, respond, error.

Request:

    {
      "original_message": "cuanto facture hoy",
      "intent": "query_billing",
      "function_name": "get_daily_billing",
      "parameters": {"date": "hoy"},
      "raw_data": "150000",
      "history": []
    }

Response:

    {
      "response": "Hoy has facturado un total de 150.000 Gs.",
      "history_entry": {
        "role": "assistant",
        "summary": "respondí sobre query_billing",
        "timestamp": "2026-10-14T10:30:00-03:00"
      },
      "mode": "respond"
    }

## Panel admin

    http://localhost:8080/admin/login

Usar las credenciales del .env. Panel en desarrollo.

## Estructura del proyecto

    gaia/
    ├── cmd/
    │   ├── app/                    → entrypoint
    │   └── hash/                   → utilidad para generar hash de contraseñas
    ├── internal/
    │   ├── admin/                  → panel web (login + dashboard)
    │   ├── brain/
    │   │   ├── brain.go            → orquestador
    │   │   ├── planner/            → Cerebro 1 (analyze)
    │   │   └── synthesizer/        → Cerebro 2 (synthesize)
    │   ├── handlers/               → endpoints HTTP
    │   ├── models/                 → structs compartidos
    │   ├── normalizer/             → normalización determinística
    │   ├── providers/
    │   │   ├── provider.go         → interfaz LLMProvider
    │   │   └── mistral/            → cliente de Mistral
    │   └── router/                 → rutas de Gin
    ├── .env                        → (NO subir a Git)
    ├── go.mod
    └── README.md

## Tests

    go test ./...
    go test ./internal/normalizer/ -v

## Estado del proyecto

- [x] Planner — análisis de intenciones
- [x] Synthesizer — 4 modos (conversation, clarify, respond, error)
- [x] Normalizer — fechas, números, monedas, booleanos
- [x] API REST stateless
- [x] Panel admin básico
- [ ] Multi-tenant (en desarrollo)
- [ ] Streaming (pendiente)
- [ ] Idempotencia (pendiente)

## Despliegue y Releases (CI/CD)

GAIA usa **GitHub Actions** para compilar automáticamente los binarios y
publicarlos como *Releases* descargables. Esto evita compilar en el servidor
de producción (ahorra RAM y tiempo).

### ¿Cómo funciona?

Existen dos workflows en `.github/workflows/`:

1. **`build.yml`**: se ejecuta en cada `git push` a `main`. El binario
   resultante queda disponible en la pestaña **Actions → Artifacts** por
   30 días. Útil para pruebas rápidas.

2. **`release.yml`**: se ejecuta **solo cuando creás un tag** (ej. `v0.1.0`).
   Genera un *Release* permanente en GitHub con el binario adjunto, listo
   para descargar desde el instalador automático.

### Publicar una nueva versión (Release)

El simple `git push` NO genera un Release descargable de forma permanente.
Debés crear un tag de versión:

    git add .
    git commit -m "feat: descripción del cambio"
    git tag v0.1.1
    git push origin main
    git push origin v0.1.1

Luego:

1. Ir a la pestaña **Actions** del repositorio.
2. Esperar ~2 minutos a que el workflow **Release GAIA** termine con ✅.
3. Ir a la pestaña **Releases** (columna derecha del repo) y verificar que
   el binario `gaia` esté adjunto a la nueva versión.

### ⚠️ Importante sobre los tags

- **Los tags son inmutables**: NO se pueden sobrescribir. Si intentás crear
  `v0.1.0` cuando ya existe, Git te dará un error.
- **Versionado semántico**: incrementá siempre el número de versión
  (`v0.1.0` → `v0.1.1` → `v0.2.0` → `v1.0.0`).
- **Si rompés un tag** (ej. lo subiste antes de tiempo):

      git tag -d v0.1.0
      git push --delete origin v0.1.0
      # corregir el problema y volver a crear el tag:
      git tag v0.1.0
      git push origin v0.1.0

### Permisos del workflow

El archivo `.github/workflows/release.yml` debe incluir el bloque de
permisos para poder crear Releases en GitHub:

    permissions:
      contents: write

Sin esto, el workflow fallará con:
`Resource not accessible by integration`.

### Flujo recomendado

| Acción | Resultado |
|--------|-----------|
| `git push` (cambios en main) | Compilación temporal en *Artifacts* (30 días) |
| `git tag vX.Y.Z && git push origin vX.Y.Z` | Release permanente con binario |
| Ejecutar el instalador en el servidor | Descarga el último Release y configura systemd |

## Contribuir

1. Crear branch: git checkout -b feature/nombre
2. Commitear: git commit -m "feat: descripción"
3. Push: git push origin feature/nombre
4. Pull request

## Licencia

