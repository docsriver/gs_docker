# gs_docker

Контейнерный GhostScript-сервис для DocsRiver: HTTP-сервис на порту **9080**
(Go-листенер), который выполняет операции с PS/PCL/PDF — конвертацию, подсчёт и
извлечение страниц. GhostScript (`gs`) и опционально GhostPCL (`gpcl6`) живут
внутри образа.

Это **контейнерный аналог** [`gs_docsriver_listener`](https://github.com/docsriver/gs_docsriver_listener):
оба реализуют один и тот же HTTP-контракт, который ждёт DocsRiver. Разница — в
способе запуска:

| | `gs_docker` | `gs_docsriver_listener` |
|---|---|---|
| Что это | Docker-контейнер | systemd-служба на хосте |
| GhostScript | внутри образа | системный (`apt install ghostscript`) |
| Сборка | `docker build` (нужен интернет) | не нужна |
| Когда удобен | обычная установка | offline / без сборки образов |

DocsRiver обращается к сервису по адресу из `HOST_URL` (по умолчанию
`http://host.docker.internal:9080/` — захардкожено в DocsRiver), поэтому
контейнер `gs` публикует порт `9080` на хост, а контейнер DocsRiver видит его
через `extra_hosts: host.docker.internal:host-gateway`.

## Состав

```
gs.Dockerfile          # multi-stage сборка
gs_listener/main.go    # HTTP-листенер на Go (порт 9080)
```

## Сборка

```shell
docker build -f gs.Dockerfile -t docsriver-gs:latest .
```

Build-аргументы:

| ARG | По умолчанию | Назначение |
|---|---|---|
| `WITH_GS` | `true` | поставить `ghostscript` (через `apk`) |
| `WITH_GPCL6` | `true` | собрать `gpcl6` из исходников ghostpdl (для PCL) |
| `GHOSTPDL_URL` | ghostpdl-10.06.0 | откуда брать исходники ghostpdl |

Примеры:

```shell
# только Ghostscript, без GhostPCL (быстрее — gpcl6 не компилируется)
docker build -f gs.Dockerfile --build-arg WITH_GPCL6=false -t docsriver-gs:latest .

# полный набор (gs + gpcl6); сборка gpcl6 из исходников ~10-20 мин
docker build -f gs.Dockerfile -t docsriver-gs:latest .
```

> Сборка `gpcl6` компилирует ghostpdl из исходников и занимает 10-20 минут.
> `ghostscript` ставится из apk-репозитория alpine и быстрый.

## Запуск

Отдельным контейнером:

```shell
docker run -d --name gs --restart always \
  -p 9080:9080 \
  -v /opt/docsriver/job_files:/data \
  docsriver-gs:latest
```

Через docker-compose (сервис в стеке DocsRiver):

```yaml
  gs:
    container_name: gs
    image: docsriver-gs:latest
    restart: always
    ports:
      - "9080:9080"
    volumes:
      - ./job_files:/data
```

`./job_files` (на хосте) — **та же папка**, которую монтирует DocsRiver. Сервис
читает/пишет файлы заданий именно там.

## Конфигурация

| Переменная | По умолчанию | Назначение |
|---|---|---|
| `DOCS_RIVER_FILES_PATH` | `/data` | папка с файлами заданий (монтируется снаружи) |

Порт фиксирован — `9080` (`EXPOSE 9080`).

## HTTP API

`POST /` с JSON-телом `{"command": "...", ...}`. Ответ — JSON
`{"status": "OK" | "ERROR", "message": "..."}`. Имена файлов в запросах —
относительно `DOCS_RIVER_FILES_PATH` (ведущий `/` отбрасывается).

| `command` | Поля запроса | Действие |
|---|---|---|
| `info` | — | `{"gs": bool, "gpcl6": bool}` — какие бинарники доступны (DocsRiver делает probe) |
| `ping` | `testfile` *(опц.)* | `pong` или содержимое `testfile` |
| `ps2pdf` | `psfile`, `pdffile` | `ps2pdf` PS → PDF |
| `pcl2pdf` | `pclfile`, `pdffile` | `gpcl6` PCL → PDF |
| `countPages` | `pdffile` | число страниц PDF |
| `extractPDFPages` | `pdffile`, `start`, `end`, `output` | вырезать страницы `start..end` в новый PDF |
| `extractPDFImagesPages` | `pdffile`, `start`, `end`, `output`, `is_monochrom` | растеризовать страницы (`pdfimage32` / `pdfimage8`) |

### Примеры

```shell
curl -s -X POST http://localhost:9080/ -d '{"command":"info"}'
# {"status": "OK", "message": "{\"gs\": true, \"gpcl6\": true}"}

curl -s -X POST http://localhost:9080/ -d '{"command":"countPages","pdffile":"job.pdf"}'
# {"status": "OK", "message": "3"}
```

## Проверка интеграции с DocsRiver

При старте DocsRiver делает `command=info` и подхватывает GhostScript. Признаки
рабочей связки:

* в логах `gs` — `POST request: command=info`;
* в логах DocsRiver — `custom-gs loaded`.

## Лицензии

GhostScript и GhostPCL распространяются под **GNU AGPL v3.0**
(<https://www.ghostscript.com/licensing/>). Образ собирается на стороне
пользователя из исходников/пакетов и не распространяется в готовом виде.
Для коммерческого использования — лицензия Artifex (<https://artifex.com/licensing/>).
