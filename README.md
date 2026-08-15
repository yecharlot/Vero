# Vero

**Identidad comercial portable** para pequeños negocios.

- Español: *“Pásame tu Vero”*
- English: *“Send me your Vero”*

No es otra red social. Es la tarjeta comercial (URL + QR + catálogo + reputación + WhatsApp) que el vendedor comparte en los canales donde ya vende.

Este repositorio es **independiente de PrismaTec / Alset Sales Hub**. Despliégalo en su propio servicio (Render, Fly, VPS) para no mezclar con otros productos.

## Arranque

```bash
go run ./cmd/vero
```

- App: http://localhost:8080/
- Perfil: http://localhost:8080/z/tu-slug
- Health: http://localhost:8080/health

```bash
docker build -t vero .
docker run -p 8080:8080 -e PORT=8080 vero
```

Datos en `vero_data/` (o `VERO_DATA_DIR`).

## MVP incluido

- Registro / login / logout
- Crear negocio con slug único e ID `VERO-…`
- Catálogo de productos
- Perfil público mobile-first
- Botón WhatsApp + mensaje prellenado
- QR del perfil
- Reseñas básicas
- Vero Score (indicador de actividad, no garantía de seguridad)
- Estadísticas: visitas, clics WhatsApp, escaneos QR

## API (resumen)

`/api/vero/auth/*` · `/api/vero/businesses` · `/api/vero/public/:slug` · `/api/vero/qr/:slug` · `/z/:slug`

## Relación con PrismaTec

PrismaTec puede seguir sirviendo Sales Hub y el runtime Alset. **Vero vive aquí**, en su propio deploy. No hace falta el nodo completo ni Supabase de PrismaTec para el MVP.


## Deploy en Render

1. Entra en [dashboard.render.com](https://dashboard.render.com).
2. **New +** → **Web Service**.
3. Conecta el repo `yecharlot/Vero`.
4. Ajustes recomendados:
   - **Runtime:** Docker (usa el `Dockerfile` del repo)
   - **Name:** `vero`
   - **Instance type:** Free
   - **Health check path:** `/health`
5. (Opcional) Disco persistente: monta `/var/data` y define `VERO_DATA_DIR=/var/data/vero` para no perder datos al redeploy.
6. **Create Web Service** y espera el build.

También puedes usar **Blueprint**: New → Blueprint → repo `yecharlot/Vero` (archivo `render.yaml`).

URL típica: `https://vero.onrender.com` (o el nombre que elijas).


## Persistencia en GitHub Gist

En Render (plan free el disco se pierde al redeploy). Opcional:

| Variable | Descripción |
|----------|-------------|
| `GITHUB_TOKEN` | Token classic con permiso **gist** |
| `VERO_GIST_ID` | ID del gist (si vacío, se crea uno secreto al primer guardado) |

El estado completo se guarda en el archivo `vero_state.json` del gist (secreto). Sirve como backup durable del MVP; no sustituye una base de datos bajo mucha concurrencia.
