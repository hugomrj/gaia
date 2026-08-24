# GAIA

GAIA es un gateway de inteligencia artificial desarrollado en Go.

Su objetivo es centralizar y controlar el acceso a diferentes proveedores y
modelos de inteligencia artificial, proporcionando APIs estandarizadas para
las aplicaciones que consumen sus servicios.

## Ejecución

Desde la raíz del proyecto:

```bash
go run ./cmd/app
```

GAIA estará disponible en:

```
http://localhost:8080
```


Compilación
```
go build -o gaia ./cmd/app
```

Para ejecutar:
```
./gaia
```

Configuración

Las variables de entorno se definen en el archivo .env.

```
GAIA_ADMIN_USER=admin
GAIA_ADMIN_PASSWORD_HASH=
```


Estado

Proyecto en desarrollo.