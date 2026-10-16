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

El Planner analiza el mensaje del usuario y devuelve la intención, la
función y los parámetros. También informa si el mensaje es conversacional.

Request:

    {
      "session_id": "abc-123",
      "message": "cuanto facture hoy",
      "function_name": "obtener_facturacion_diaria",
      "data_slots": {"fecha": null},
      "history": []
    }

Los campos `function_name` y `data_slots` son opcionales. Se usan cuando
el cliente ya inició una conversación con una función pendiente que
espera datos (slot-filling).

Response:

    {
      "intent": "El usuario quiere saber cuánto se facturó hoy",
      "is_conversational": false,
      "function_name": "obtener_facturacion_diaria",
      "parameters": {"fecha": "hoy"},
      "confidence": 0.95,
      "original_input": "cuanto facture hoy"
    }

Campos:

- `intent` — frase en castellano que describe qué quiere el usuario.
- `is_conversational` — `true` si es charla casual (saludo, agradecimiento,
  despedida, info del bot, mi perfil). `false` en cualquier otro caso.
- `function_name` — nombre de función en castellano, snake_case.
- `parameters` — claves en castellano, valores crudos sin normalizar.
- `confidence` — 0.0 a 1.0.

Ejemplo conversacional:

    {
      "intent": "El usuario saluda",
      "is_conversational": true,
      "function_name": "obtener_saludo",
      "parameters": {},
      "confidence": 1.0,
      "original_input": "hola"
    }






### POST /v1/normalize

Convierte valores crudos a formato estándar (opcional). Determinístico,
sin LLM. El cliente decide si lo usa o no.

Request:

    {
      "parameters": {
        "fecha": "el lunes",
        "monto": "150 mil",
        "periodo": "octubre de 2025"
      }
    }

Response:

    {
      "parameters": {
        "fecha": "2026-10-19",
        "monto": 150000,
        "periodo": {"from": "2025-10-01", "to": "2025-10-31"}
      },
      "normalized": ["fecha", "monto", "periodo"],
      "not_normalized": []
    }

Casos que maneja:

- **Fechas:** `hoy`, `ayer`, `hace 3 días`, `el lunes`, `15 de octubre`,
  `15/10/2026`, `2026-10-15`.
- **Períodos:** `este mes`, `el mes pasado`, `octubre`, `2026`,
  `octubre de 2025`. Devuelve `{from, to}`.
- **Números:** `150 mil` → `150000`, `1.5M` → `1500000`,
  `1.500,50` → `1500.50` (formato europeo).
- **Monedas:** `150000 Gs` → `150000`.
- **Booleanos:** `sí`, `no`, `true`, `false` (solo si el campo lo sugiere
  por su nombre: `is_active`, `tiene_stock`).
- **Cédulas:** se preservan como string, sin notación científica.

Los campos que no se pudieron normalizar aparecen en `not_normalized`.




### POST /v1/synthesize

El Synthesizer redacta la respuesta al usuario. Tiene 4 modos automáticos:
`conversation`, `clarify`, `respond`, `error`.

Request:

    {
      "original_message": "cuanto facture hoy",
      "intent": "El usuario quiere saber cuánto se facturó hoy",
      "is_conversational": false,
      "function_name": "obtener_facturacion_diaria",
      "parameters": {"fecha": "hoy"},
      "data_slots": {},
      "raw_data": "{\"type\":\"single\",\"label\":\"facturación del día\",\"data\":7075000,\"meta\":{\"unit\":\"Gs\"}}",
      "confidence": 0.95,
      "history": []
    }

Response:

    {
      "response": "Hoy facturaste 7.075.000 Gs.",
      "history_entry": {
        "role": "assistant",
        "summary": "obtener_facturacion_diaria(fecha=hoy)",
        "timestamp": "2026-10-15T10:30:00-03:00"
      },
      "mode": "respond",
      "data_slots": {},
      "raw_data": "{\"type\":\"single\",...}"
    }

Campos del request:

- `is_conversational` — determina si el modo es `conversation`.
- `data_slots` — si hay claves con `null`, el modo es `clarify`.
- `raw_data` — envelope JSON (ver siguiente sección). Si está vacío y no
  hay slots nulos, el modo es `conversation`.
- `error_code` — si está presente, el modo es `error`.

El `history_entry.summary` guarda la función y sus parámetros en formato
`funcion(param=valor)`. El cliente debe persistirlo tal cual y enviarlo
en el siguiente request. El Planner lo usa para continuidad.

**Modos:**

| Modo | Cuándo |
|---|---|
| `conversation` | `is_conversational: true` |
| `clarify` | hay slots en null o confianza baja |
| `respond` | hay `raw_data` válido |
| `error` | hay `error_code` |




### Slot-filling con `data_slots`

`data_slots` es el estado de los datos que el usuario debe proporcionar
para que una función pueda ejecutarse. Las claves las define el contrato
de cada función en el cliente.

Ejemplo con `agendar_reunion` (requiere `doctor` y `fecha`):

Turno 1:

    POST /v1/analyze
    {"message": "quiero agendar una reunion"}

    →
    {
      "function_name": "agendar_reunion",
      "parameters": {},
      "is_conversational": false
    }

El cliente sabe que `agendar_reunion` requiere `{doctor, fecha}`. Arma
`data_slots: {doctor: null, fecha: null}` y pide los datos al usuario.

Turno 2:

    POST /v1/analyze
    {
      "message": "con el doctor ramirez",
      "function_name": "agendar_reunion",
      "data_slots": {"doctor": null, "fecha": null}
    }

    →
    {
      "function_name": "agendar_reunion",
      "parameters": {"doctor": "ramirez"}
    }

El Planner respeta la función pendiente y asigna el valor al slot
correcto. El cliente fusiona y actualiza `data_slots`.

Cuando `data_slots` no tiene nulls, el cliente ejecuta la función.




## Formato de datos del cliente (raw_data)

El campo `raw_data` del Synthesizer es la respuesta del sistema empresarial
hacia GAIA. Es la única vía por la que GAIA recibe información real.
Para que el Synthesizer redacte bien, el cliente debe enviarlo en un
formato uniforme.

### Envelope estándar

El cliente envía siempre un objeto JSON con esta estructura:

    {
      "type": "single" | "list" | "empty" | "error",
      "label": "descripción corta de qué es el dato",
      "data": <valor o lista o null>,
      "meta": { ... }   // opcional
    }

### Reglas

1. **Siempre JSON.** Nunca texto plano.
2. **Siempre `label`.** Es lo que le dice al Synthesizer QUÉ es el dato.
3. **`data` semántico.** Si es lista, cada item con claves claras.
4. **`meta` opcional.** Para `unit`, `count`, `fecha`, `periodo`.
5. **Nunca inventes campos.** Si no aplica, omitilo.

### Tipos

#### `single` — un solo valor

    {
      "type": "single",
      "label": "facturación del día",
      "data": 7075000,
      "meta": { "unit": "Gs" }
    }

Si el valor tiene varias partes:

    {
      "type": "single",
      "label": "resumen de facturación del mes",
      "data": { "total": 7075000, "facturas": 522 },
      "meta": { "unit": "Gs", "periodo": "2026-10" }
    }

#### `list` — múltiples registros

    {
      "type": "list",
      "label": "clientes en Asunción",
      "data": [
        { "nombre": "Juan Pérez", "deuda": 150000 },
        { "nombre": "María González", "deuda": 200000 }
      ],
      "meta": { "count": 2 }
    }

#### `empty` — sin resultados

    {
      "type": "empty",
      "label": "clientes en Marte",
      "data": null
    }

#### `error` — fallo controlado

    {
      "type": "error",
      "label": "consulta de facturación",
      "data": null,
      "meta": { "message": "timeout db" }
    }

### Ejemplo completo del flujo

1. El Planner devuelve `obtener_facturacion_diaria` con `{"fecha":"hoy"}`.
2. El cliente ejecuta la función real.
3. El cliente arma el envelope:

        {
          "type": "single",
          "label": "facturación del día",
          "data": 7075000,
          "meta": { "unit": "Gs", "fecha": "2026-10-15" }
        }

4. Lo envía como `raw_data` (string JSON) al Synthesizer.
5. El Synthesizer responde:

        "Hoy facturaste 7.075.000 Gs."

### Qué NO enviar

- Texto libre: `"La facturación de hoy fue 7075000 Gs."`
- Objetos sin `label`: `{"data": 7075000}`
- JSON sin `type`: `{"total": 7075000}`

Aunque el Synthesizer es tolerante a estos formatos (modo legacy), el
envelope estándar es el único que garantiza que el LLM entienda qué es
cada dato sin ambigüedad.

### Relación con el historial

El `history_entry` que devuelve el Synthesizer guarda:

    "obtener_facturacion_diaria(fecha=hoy)"

El cliente debe persistirlo tal cual en el historial y enviarlo en el
siguiente request. El Planner lo usa para inferir continuidad
("y ayer?" → `fecha=ayer`).





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

