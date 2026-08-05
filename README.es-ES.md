

# Respondent

> Inteligencia geoespacial en tiempo real en un globo 3D.

Respondent ingiere datos en vivo de más de 50 fuentes públicas — vuelos, barcos, terremotos, incendios, rayos, satélites, eventos de conflicto, alertas meteorológicas, interrupciones de internet y más — y los representa en un globo 3D interactivo. Los análisis opcionales impulsados por IA fusionan datos entre capas para mostrar señales compuestas (p. ej. actividad militar cerca de zonas de conflicto, barcos cerca de cables submarinos, incendios cerca de anomalías en la calidad del aire).

📚 **Documentación completa: [respondent-docs.alevsk.dev](https://respondent-docs.alevsk.dev)** — referencia de esquemas, guías de autoría de fuentes/análisis y detalles de configuración.

Este repositorio distribuye la Edición Comunitaria: un único contenedor Docker, una base de datos SQLite y un directorio de definiciones en YAML de fuentes y análisis que puedes editar, ampliar y contribuir de vuelta.

<p align="center" width="100%">
  <video src="https://github.com/user-attachments/assets/22111890-6e4a-4d7b-9ca4-9d4cad79ccd5" width="80%" controls></video>
</p>

---

## Inicio Rápido

Necesitas [Docker](https://docs.docker.com/get-docker/) (v20.10+) y [Docker Compose](https://docs.docker.com/compose/install/) (v2.0+).

```bash
# 1. Clona este repositorio
git clone https://github.com/alevsk/respondent-community.git
cd respondent-community

# 2. (Opcional) copia la plantilla de entorno y completa las claves API que tengas
cp .env.example .env
$EDITOR .env

# 3. Inicia Respondent
docker compose up -d

# 4. Abre el globo
open http://localhost:8090   # macOS — o visita la URL en cualquier navegador
```

Eso es todo. El contenedor descarga `docker.io/alevsk/respondent-community:latest`, monta los directorios locales `respondent.yaml`, `sources.d/` y `analysis.d/` de solo lectura, y persiste la base de datos SQLite en un volumen Docker llamado `respondent_data`.

Para ver el proceso de ingestión:

```bash
docker compose logs -f
```

Para detener:

```bash
docker compose down            # detener, conservar datos
docker compose down -v         # detener, eliminar el volumen de datos (destructivo)
```

### Alternativa: `docker run`

Docker Compose es la ruta **recomendada**. Si no puedes usarlo, el comando equivalente `docker run` es:

```bash
docker volume create respondent_data

docker run -d \
  --name respondent-community \
  --restart unless-stopped \
  --env-file ./.env \
  -e RESPONDENT_DATABASE_PATH=/data/respondent.db \
  -p 8090:8090 \
  -v "$(pwd)/respondent.yaml:/etc/respondent/respondent.community.yaml:ro" \
  -v "$(pwd)/sources.d:/etc/respondent/sources.d:ro" \
  -v "$(pwd)/analysis.d:/etc/respondent/analysis.d:ro" \
  -v respondent_data:/data \
  docker.io/alevsk/respondent-community:latest
```

Luego abre el globo en **http://localhost:8090**.

---

## Qué está incluido

- **Despliegue en un solo contenedor** — una imagen, un proceso, SQLite para almacenamiento. No se requiere Postgres, Redis ni Kafka.
- **Interfaz de globo 3D** — frontend en React + Cesium, servido desde el mismo puerto que la API y el WebSocket.
- **Más de 30 fuentes de datos públicas** listas para usar, incluyendo vuelos (ADS-B, OpenSky), barcos (AIS), terremotos (USGS, EMSC), incendios (NASA FIRMS, NIFC), rayos (Blitzortung), satélites (CelesTrak, TLE API), volcanes, alertas meteorológicas, calidad del aire, eventos de conflicto, infraestructura de internet, indicadores de mercados financieros, la EEI y más. Consulta [`sources.d/README.md`](./sources.d/README.md) para el catálogo completo.
- **Análisis con IA** — pipelines declarativos en YAML que se ejecutan según un horario, consultan una o más capas, envían un prompt a un LLM con datos estructurados y almacenan los hallazgos en la base de datos. Los ejemplos incluyen predicción de incendios por rayos, detección de amenazas a cables marítimos y un índice de puntos calientes geopolíticos. Consulta [`analysis.d/README.md`](./analysis.d/README.md) para el catálogo completo.
- **No se requiere código** para agregar una fuente o análisis — todo es YAML + expresiones CEL.

---

## Configuración

Dos archivos controlan el comportamiento en tiempo de ejecución:

| Archivo | Propósito |
|---|---|
| `respondent.yaml` | Configuración de ejecución: puerto del servidor, ruta de la base de datos, activación de IA, proveedor de LLM, geocodificador. Montado de solo lectura en el contenedor en `/etc/respondent/respondent.community.yaml`. |
| `.env` | Secretos y claves API por fuente. Cargados automáticamente por Docker Compose si están presentes. Ignorados por Git. |

Los valores predeterminados en `respondent.yaml` funcionan listas para usar: las fuentes que necesitan credenciales se omiten silenciosamente cuando falta su clave, por lo que puedes comenzar con cero variables de entorno y agregar claves a medida que avances.

Para la referencia completa del esquema (cada campo, cada tipo de transporte de fuente, cada formato de analizador), consulta la documentación para desarrolladores: navega en línea en <https://respondent-docs.alevsk.dev> o ejecútala localmente:

```bash
docker compose -f developer-documentation/compose.yaml up -d
# Luego abre http://localhost:8080
```

El mismo contenido también está disponible en [`developer-documentation/content/`](./developer-documentation/content/) como Markdown plano.

---

## Claves API (Opcional)

La mayoría de las fuentes funcionan sin credenciales. Las siguientes fuentes requieren una clave API o token gratuito para ingerir datos: sin él, se iniciarán, fallarán en la autenticación y se detendrán hasta que proporciones una clave. Ninguna es obligatoria para una instalación funcional.

| Fuente | Dónde obtenerla | Variable(s) de entorno |
|---|---|---|
| Incendios activos de NASA FIRMS | <https://firms.modaps.eosdis.nasa.gov/api/area/> | `RESPONDENT_NASA_FIRMS_MAP_KEY` |
| Calidad del aire de OpenAQ | <https://docs.openaq.org/> | `RESPONDENT_OPENAQ_API_KEY` |
| AIS marítimo de AISStream | <https://aisstream.io/> | `RESPONDENT_AISSTREAM_APY_KEY` |
| Calidad del aire comunitaria de PurpleAir | <https://develop.purpleair.com/> | `RESPONDENT_PURPLEAIR_API_KEY` |
| Eventos de conflicto armado de ACLED | <https://acleddata.com/> | `RESPONDENT_ACLED_EMAIL`, `RESPONDENT_ACLED_PASSWORD` |
| Radioaficionados de APRS.fi | <https://aprs.fi/> | `RESPONDENT_APRS_FI_API_KEY` |
| Interrupciones de internet de Cloudflare Radar | <https://radar.cloudflare.com/> | `RESPONDENT_CLOUDFLARE_RADAR_TOKEN` |
| Malla LoRa de Meshtastic | <https://meshtastic.org/> (las credenciales públicas funcionan) | `RESPONDENT_MESHTASTIC_USER`, `RESPONDENT_MESHTASTIC_PASS` |
| Alertas de ataque aéreo de Ucrania | <https://alerts.in.ua/> | `RESPONDENT_UKRAINE_ALARM_TOKEN` |

Agrega las variables que tengas a `.env` (copia `.env.example` para obtener la lista completa anotada, incluidas las claves del proveedor de LLM).

Si también deseas imágenes satelitales en el globo (Bing Maps Aerial mediante Cesium Ion), obtén un token gratuito en <https://ion.cesium.com/signup> y configura:

```bash
RESPONDENT_FRONTEND_CESIUM_ION_TOKEN=your-token-here
```

Sin él, el globo recurre a las teselas oscuras de Stadia Maps.

---

## Habilitar análisis con IA

La IA está deshabilitada por defecto. Para habilitarla, edita `respondent.yaml`:

```yaml
ai:
  enabled: true

llm:
  provider: "openai"     # or anthropic, xai, gemini, zai, ollama, lmstudio
  openai:
    model: "gpt-4o"
    max_tokens: 2048
```

…y configura la clave API correspondiente en `.env`:

```bash
RESPONDENT_LLM_OPENAI_API_KEY=sk-...
```

Luego reinicia:

```bash
docker compose restart
```

Los análisis definidos en `analysis.d/` se recogerán automáticamente y comenzarán a ejecutarse según sus horarios configurados.

---

## Actualización

```bash
docker compose pull
docker compose up -d
```

Tu volumen de datos (`respondent_data`) y los archivos de configuración se conservan.

---

## Documentación

- **Documentación para usuarios finales / operadores**: este README y las guías de [Primeros Pasos](./developer-documentation/content/getting-started/).
- **Referencia de esquemas** (cada campo de cada YAML): [`developer-documentation/`](./developer-documentation/) — sitio estático en Hugo, también publicado en <https://respondent-docs.alevsk.dev>.
- **Catálogo de fuentes**: [`sources.d/README.md`](./sources.d/README.md)
- **Catálogo de análisis**: [`analysis.d/README.md`](./analysis.d/README.md)
- **Plantillas**: [`sources.d/TEMPLATE.yaml`](./sources.d/TEMPLATE.yaml), [`analysis.d/TEMPLATE.yaml`](./analysis.d/TEMPLATE.yaml)

---

## Contribuciones

Se aceptan pull requests para nuevas fuentes, nuevos análisis y mejoras en la documentación. Consulta [DEVELOPMENT.md](./DEVELOPMENT.md) para la guía completa de contribución: configuración local, los esquemas YAML, cómo probar y las convenciones de PR.

---

## Licencia

Por definir. Se agregará un archivo `LICENSE` antes del primer lanzamiento etiquetado. Hasta entonces, trata este repositorio como "todos los derechos reservados" por los autores originales; puedes ejecutarlo localmente y enviar contribuciones, pero aún no se otorgan derechos de redistribución.

---

## Obtener Ayuda

- Reportar un problema: <https://github.com/alevsk/respondent-community/issues>
- Documentación: <https://respondent-docs.alevsk.dev>
