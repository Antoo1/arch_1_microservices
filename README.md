# Project_template

Это шаблон для решения проектной работы. Структура этого файла повторяет структуру заданий. Заполняйте его по мере работы над решением.

# Задание 1. Анализ и планирование

<aside>

Чтобы составить документ с описанием текущей архитектуры приложения, можно часть информации взять из описания компании и условия задания. Это нормально.

</aside>

### 1. Описание функциональности монолитного приложения

**Управление отоплением:**
- На данный момент исходя из кода приложения управление отоплением не реализовано, хоть в описании оно существует.
- В описании: пользователи могут удалённо включать/выключать отопление в своих домах.
- В коде: этого нет. В модели данных есть только один тип устройства - `temperature` (`models/sensor.go`), команд включения/выключения отопления нет.

**Управление датчиками:**

- В описании: умный дом позволяет управлять датчиками, а датчики при подключении регистрируются.
- В коде: управление датчиками реализовано - CRUD через REST (`handlers/sensors.go`, `db/db.go`).
- В коде: есть создание записи о датчике через `POST /api/v1/sensors` (`CreateSensor`). Кто вызывает этот эндпоинт, в коде не определено - это может быть оператор, веб-клиент или сам датчик; аутентификации и идентификации вызывающего нет.
- В коде: специальной логики авто-регистрации «при подключении» (pairing, handshake, привязка к владельцу) нет - только обычная вставка строки по REST-запросу.

**Мониторинг температуры:**

- Пользователь получает текущую температуру через REST API (`GET /api/v1/sensors`, `GET /api/v1/sensors/:id`, `GET /api/v1/sensors/temperature/:location`).
- Данные о температуре сервер забирает сам (pull) HTTP-запросом к внешнему источнику (`services/temperature_service.go`). Push от датчика отсутствует.
- CRUD сенсоров: создание, чтение, обновление, удаление, обновление значения/статуса (`handlers/sensors.go`).

### 2. Анализ архитектуры монолитного приложения

- **Язык:** Go (`go.mod`).
- **СУБД:** PostgreSQL, через `pgx` (`db/db.go`).
- **Архитектура:** монолит. Обработка HTTP (gin), бизнес-логика и доступ к данным - в одном приложении и одном процессе (`main.go`).
- **Взаимодействие:** синхронное. Все вызовы блокирующие, очередей/брокеров/событий нет. Внешний источник температуры опрашивается синхронным HTTP `GET`.
- **Модель опроса датчиков:** pull - инициатор всегда сервер.
- **Аутентификация/авторизация:** отсутствует, все эндпоинты публичны.
- **Тесты:** отсутствуют.
- **Масштабируемость:** ограничена - масштабируется только всё приложение целиком, по доменам отдельно - нельзя.
- **Развёртывание:** единый деплой, обновление требует пересборки и перезапуска всего приложения.

### 3. Определение доменов и границы контекстов

Домены, выделенные из текущего кода и описания:

- **Управление устройствами (device_management).** Регистрация, конфигурация, CRUD сенсоров/реле. В коде: `handlers/sensors.go`, `db/db.go`, таблица `sensors`.
- **Мониторинг (telemetry).** Сбор и отдача значений температуры. В коде: `services/temperature_service.go`, поля `value/status/last_updated`.
- **Управление системой (heating_control, gate_control, light_control).** В описании есть, в коде не реализован.

В текущем монолите границы контекстов не разделены: все домены работают через одну модель `Sensor` и одну таблицу `sensors`.

### **4. Проблемы монолитного решения**

- **Единая модель и таблица на все домены.** Любой новый тип устройства (свет, ворота, камеры) ломает общую схему `sensors`.
- **Невозможность частичного масштабирования и независимого деплоя.** Изменение одного домена требует пересборки и перезапуска всего монолита.
- **Только синхронный pull.** Датчик не может сам прислать событие. Реактивные сценарии («стало холодно → включить отопление») невозможны.
- **N+1 запросов к внешнему API.** GetSensors делает по одному HTTP-запросу к temperature-api на каждый датчик: на 100 датчиках - 100 последовательных вызовов.
- **Нет границ безопасности.** Отсутствует аутентификация и привязка устройства к владельцу - несовместимо с SaaS-самообслуживанием.
- **Нет тестов.** Любой рефакторинг рискован.
- **Заявленный функционал не покрыт.** Управление отоплением описано, но в коде отсутствует.

### 5. Визуализация контекста системы - диаграмма С4

```markdown
[C4 context diagram as is](docs/c4/context_as_is.puml)
```

### 6. Трассировка As-Is -> To-Be (мост к Заданию 2)

Таблица связывает каждый элемент существующей системы с его местом в целевой
архитектуре - чтобы переход был явным и проверяемым, а не «спроектированным с нуля».

**Тип перехода:** `replace` - заменяется фасадом/маршрутизацией · `rewrite` - переписывается
как новый сервис · `split` - расщепляется · `move` - переезжает с минимальными правками ·
`new` - в коде нет, создаётся · `retire` - временно живёт, затем выводится.

| As-Is (что есть / где) | To-Be (куда) | Тип |
|---|---|---|
| REST API + маршруты монолита ([main.go](apps/smart_home/main.go)) | API Gateway + доменные сервисы | replace (фасад) |
| Sensor CRUD ([handlers/sensors.go](apps/smart_home/handlers/sensors.go)) + таблица `sensors` ([init.sql](apps/smart_home/init.sql)) | Device Management + `devices` | rewrite + миграция данных |
| Поля `value/status/last_updated`, отдача показаний | Telemetry + `telemetry_data` | rewrite |
| `TemperatureService` - pull HTTP-клиент ([temperature_service.go](apps/smart_home/services/temperature_service.go)) | Poller-адаптер в Connectivity Gateway | move/reshape |
| `temperature-api` - стаб (Task 5, [docker-compose.yml](apps/docker-compose.yml)) | остаётся как legacy pull-источник, таргет поллера | keep -> retire |
| Одна БД PostgreSQL | БД-на-сервис + TimescaleDB для телеметрии | split |
| Аутентификация (в коде нет) | Identity | new |
| Свет / ворота / наблюдение / сценарии (в описании есть, в коде нет) | Control + Observation + Scenario | new |

Два шва перехода:

- **South (данные).** `TemperatureService -> temperature-api` (синхронный pull inline) превращается
  в `Connectivity Gateway (поллер) -> temperature-api -> Event Bus -> Telemetry`. Здесь же чинится
  N+1 из `GetSensors`.
- **North (клиенты).** Закрывается фасадом на API Gateway: трафик клиентов сначала идёт в монолит,
  затем по мере готовности сервисов переключается на новые - контракт клиента остаётся стабильным.
  Монолит при этом не правится, а «усыхает» эндпоинт за эндпоинтом (strangler).

### 7. План перехода (фазы) - strangler

Переход выполняется итеративно. Инвариант: на каждой фазе клиентский контракт стабилен.

| Фаза | Состояние | Клиент видит |
|---|---|---|
| 0 (as-is) | клиент -> монолит -> temperature-api (pull inline, N+1) | - |
| 1 (фасад + инфра) | API GW перед монолитом (100% трафика -> монолит). Подняты Event Bus + Connectivity GW (поллер) + Telemetry. Поллер опрашивает temperature-api -> шина -> Telemetry (**shadow**, параллельно) | без изменений |
| 2 (срезаем чтение) | GW роутит `GET /sensors` -> Telemetry / Device Management. Монолит больше не делает inline-pull -> **N+1 устранён** | без изменений |
| 3 (срезаем запись) | GW роутит CRUD -> Device Management. Одноразовая миграция данных `sensors` -> `devices` | без изменений |
| 4 (вывод) | Монолит удалён. temperature-api живёт, пока его не заменят реальные MQTT-устройства -> затем гасится поллер-адаптер | без изменений |

**Диаграммы переходного периода:**

- [transition_containers.puml](docs/c4/transition_containers.puml) - сосуществование монолита и новых сервисов за фасадом API Gateway.
- [transition_sequence.puml](docs/c4/transition_sequence.puml) - обслуживание `GET /sensors` в overlap (до и после миграции пути) + shadow-поток поллера.


# Задание 2. Проектирование микросервисной архитектуры

В этом задании вам нужно предоставить только диаграммы в модели C4. Мы не просим вас отдельно описывать получившиеся микросервисы и то, как вы определили взаимодействия между компонентами To-Be системы. Если вы правильно подготовите диаграммы C4, они и так это покажут.

Ось декомпозиции - **по роли устройства** (сенсор / исполнитель) + автоматизация отдельно, а не по типу прибора. Вариация «вендор/модель» вынесена в **device-профили** (данные в Device Management), а не в код/сервисы - это закрывает требование «подключение ещё неизвестных устройств». Транспорт изолирован в **протокольных адаптерах** Connectivity Gateway (north-bound generic Control + south-bound адаптеры).

Микросервисы: API Gateway, Identity, Device Management, Telemetry, Control, Scenario/Automation, Наблюдение (Observation), Device Connectivity Gateway, Event Bus. БД - per-service, телеметрия в TimescaleDB.

**Диаграмма контейнеров (Containers)**

[c4_containers.puml](docs/c4/containers.puml)

**Диаграмма компонентов (Components)**

- Device Management: [c4_components_device_mgmt.puml](docs/c4/components_device_mgmt.puml) - реестр устройств + profile/capability registry.
- Control: [c4_components_control.puml](docs/c4/components_control.puml) - generic north-bound: резолв профиля, desired/reported state, диспетчеризация в шину.
- Scenario/Automation: [c4_components_scenario.puml](docs/c4/components_scenario.puml) - подписка на телеметрию (хореография) + локальная оркестрация команд.
- Device Connectivity Gateway: [c4_components_conngw.puml](docs/c4/components_conngw.puml) - south-bound: MQTT-адаптер + HTTP-push-адаптер + **Poller/Scheduler** -> общий Normalizer/Dedup -> Bus Publisher. Поллер - наследник `TemperatureService`, точка интеграции pull-устройств и legacy `temperature-api`.

Остальные сервисы (Identity, Telemetry, Наблюдение) следуют тому же паттерну: API -> доменная логика -> repository/(de)сериализация шины.

**Диаграмма кода (Code)**

[c4_code_command_sequence.puml](docs/c4/code_command_sequence.puml) - sequence самого критичного потока: реактивный сценарий «влажность -> вентиляция» (south-bound телеметрия -> хореография-триггер -> оркестрация команды -> доставка через адаптер -> reported state).

# Задание 3. Разработка ER-диаграммы

[er_warmhouse.puml](docs/er/er_warmhouse.puml) - логическая модель данных To-Be системы.

**Ключевое решение - схема per-service, а не единая БД.** Архитектура из Задания 2 использует
БД-на-сервис, поэтому ER-диаграмма разбита по сервисам (каждая БД - отдельный `package`), а не
сведена в одну плоскую схему. Из этого следуют два типа связей:

- **── solid** - физический внешний ключ внутри одной БД сервиса.
- **·· dashed (LOGICAL FK)** - логическая ссылка на сущность *другого* сервиса. Физического FK
  нет (базы разные); целостность держится в приложении / на bind-time (см. Binding Validator в
  `components_scenario.puml`). Ключи - `UUID`, чтобы не было коллизий между сервисами.

**Сущности по сервисам:**

| Сервис (БД) | Сущности | Назначение |
|---|---|---|
| Identity (PostgreSQL) | `users`, `houses` | Аккаунты и владение. Дом принадлежит одному владельцу (1:N). |
| Device Management (PostgreSQL) | `device_types`, `device_profiles`, `rooms`, `devices` | Реестр устройств. `device_profiles` (vendor/model + `capabilities` JSONB) - механизм подключения ещё неизвестных приборов: новый прибор = новый профиль-данные, без изменения кода. |
| Telemetry (TimescaleDB) | `telemetry_data` | Time-series показаний, hypertable, без FK. |
| Control (PostgreSQL) | `device_state`, `commands` | desired/reported state (1:1 с устройством) и журнал команд. |
| Scenario (PostgreSQL) | `scenarios`, `rule_conditions`, `rule_actions` | Пользовательские правила: триггер -> условия -> действия (реактивный поток «телеметрия -> команда»). |
| Observation (PostgreSQL) | `cameras` | Метаданные камер удалённого наблюдения. |

**Основные связи:** User 1-N House · House 1-N Room/Device · DeviceType/DeviceProfile 1-N Device ·
Device 1-1 DeviceState · Device 1-N Command/TelemetryData · Scenario 1-N Condition/Action.
Сущности `Module` из примера задания в модели нет намеренно: его роль («подключение неизвестных
устройств») в этой архитектуре играют `device_profiles`/`capabilities`, а не отдельная таблица.

# Задание 4. Создание и документирование API

### 1. Тип API

Используются **два типа API** - это прямое следствие архитектуры из задания 2, которая
разделяет синхронные REST-вызовы и асинхронный обмен через Event Bus:

- **REST** - синхронное «запрос-ответ», где нужен немедленный ответ. Покрывает
  два вида взаимодействия:
  - **синхронный клиентский API** - клиент -> API Gateway -> сервисы (регистрация устройства,
    чтение состояния и показаний, отправка команды, создание сценария);
  - **синхронный межсервисный API** - сервис -> сервис (Control -> Device Management за
    профилем/capabilities, Scenario -> Control для оркестрации команды).
- **AsyncAPI** - асинхронный обмен через Event Bus (Kafka): телеметрия и команды, где
  немедленный ответ не нужен или невозможен (устройство может быть офлайн).

**Мост sync↔async.** Команда устройству показывает оба стиля в связке: REST `POST
/devices/{id}/commands` принимает команду и сразу отвечает `202 Accepted`, не дожидаясь железа;
реальная доставка идёт событием `device.command` в шину, а результат возвращается событием
`command.ack` и виден через REST `GET /commands/{id}`. `command_id` - корреляция между REST и
событиями (см. поток в [code_command_sequence.puml](docs/c4/code_command_sequence.puml)).


### 2. Документация API

Спецификации - в [docs/api/](docs/api/). Покрыт реактивный поток «влажность -> вентиляция»:
7 REST-эндпоинтов в 4 сервисах + 3 канала Event Bus.

**REST (OpenAPI):**

| Сервис | Спецификация | Эндпоинты |
|---|---|---|
| Device Management | [device-management.openapi.yaml](docs/api/device-management.openapi.yaml) | `POST /devices`, `GET /devices/{id}` |
| Control | [control.openapi.yaml](docs/api/control.openapi.yaml) | `POST /devices/{id}/commands`, `GET /devices/{id}/state`, `GET /commands/{id}` |
| Telemetry | [telemetry.openapi.yaml](docs/api/telemetry.openapi.yaml) | `GET /devices/{id}/telemetry` |
| Scenario | [scenario.openapi.yaml](docs/api/scenario.openapi.yaml) | `POST /scenarios` |

**Async (AsyncAPI):**

| Спецификация | Каналы |
|---|---|
| [event-bus.asyncapi.yaml](docs/api/event-bus.asyncapi.yaml) | `telemetry.received`, `device.command`, `command.ack` |

# Задание 5. Работа с docker и docker-compose

Перейдите в apps.

Там находится приложение-монолит для работы с датчиками температуры. В README.md описано как запустить решение.

Вам нужно:

1) сделать простое приложение temperature-api на любом удобном для вас языке программирования, которое при запросе /temperature?location= будет отдавать рандомное значение температуры.

Locations - название комнаты, sensorId - идентификатор названия комнаты

```
	// If no location is provided, use a default based on sensor ID
	if location == "" {
		switch sensorID {
		case "1":
			location = "Living Room"
		case "2":
			location = "Bedroom"
		case "3":
			location = "Kitchen"
		default:
			location = "Unknown"
		}
	}

	// If no sensor ID is provided, generate one based on location
	if sensorID == "" {
		switch location {
		case "Living Room":
			sensorID = "1"
		case "Bedroom":
			sensorID = "2"
		case "Kitchen":
			sensorID = "3"
		default:
			sensorID = "0"
		}
	}
```

2) Приложение следует упаковать в Docker и добавить в docker-compose. Порт по умолчанию должен быть 8081

3) Кроме того для smart_home приложения требуется база данных - добавьте в docker-compose файл настройки для запуска postgres с указанием скрипта инициализации ./smart_home/init.sql

Для проверки можно использовать Postman коллекцию smarthome-api.postman_collection.json и вызвать:

- Create Sensor
- Get All Sensors

Должно при каждом вызове отображаться разное значение температуры

Ревьюер будет проверять точно так же.


