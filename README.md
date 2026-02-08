# Privacy Hub

Высокопроизводительная система обеспечения приватности сетевого трафика на основе микросервисной архитектуры. Проект реализует комплексное решение для контроля DNS-запросов, HTTP/HTTPS трафика с поддержкой transparent proxy через динамическое управление iptables и оркестрацию контейнеризированных компонентов.

## Технологический стек

### Языки и фреймворки

- **Go 1.24+** — основной язык разработки
- **Docker** — контейнеризация DNS-компонента
- **Docker Compose** — оркестрация многоконтейнерного окружения

### Ключевые библиотеки и зависимости

- `github.com/miekg/dns` — полнофункциональная реализация DNS протокола
- `github.com/elazarl/goproxy` — HTTP/HTTPS proxy с MITM capabilities
- `github.com/docker/docker/client` — программное управление Docker Engine API
- `github.com/go-chi/chi/v5` — легковесный и производительный HTTP роутер
- `gopkg.in/yaml.v3` — парсинг конфигурационных файлов

### Системные компоненты

- **iptables** — динамическая маршрутизация сетевого трафика на уровне ядра
- **TLS 1.2+** — шифрованная передача данных
- **netfilter/NAT** — прозрачное перенаправление DNS-запросов

## Архитектурные паттерны

### Реализованные паттерны проектирования

#### Supervisor Pattern

Центральный компонент-супервизор управляет жизненным циклом всех подсистем с graceful shutdown и автоматическим восстановлением после сбоев.

#### Singleton Pattern

Глобальный логгер инициализируется один раз через `sync.Once` для thread-safe доступа из всех модулей.

#### Strategy Pattern

Резолвер DNS поддерживает множественные стратегии upstream провайдеров (DoT, DoH) с автоматическим fallback.

#### Decorator Pattern

Middleware цепочка для HTTP API (логирование, восстановление после паники, таймауты).

#### Cache-Aside Pattern

LRU-кеш с TTL для DNS-ответов минимизирует латентность и нагрузку на upstream серверы.

#### Resource Pool Pattern

Переиспользование TCP соединений через `sync.Pool` для DNS-клиентов.

## Системная архитектура

```mermaid
graph TB
    subgraph "Host System"
        User[User Application]
        IPT[iptables NAT Rules]
        Host[Privacy Hub Main Process]
    end
    
    subgraph "Docker Container"
        DNS[DNS Resolver]
        Cache[LRU Cache]
        Filter[Domain Filter]
    end
    
    subgraph "External Services"
        Proxy[HTTP/HTTPS Proxy<br/>:3128]
        API[REST API<br/>:8000]
    end
    
    subgraph "Upstream Providers"
        DoT[DNS-over-TLS<br/>1.1.1.1:853]
        DoH[DNS-over-HTTPS<br/>cloudflare-dns.com]
    end
    
    User -->|DNS Query :53| IPT
    IPT -->|Redirect :9000| DNS
    DNS --> Filter
    Filter -->|Blocked| User
    Filter -->|Allowed| Cache
    Cache -->|Cache Miss| DoT
    Cache -->|Fallback| DoH
    Cache -->|Cache Hit| User
    
    User -->|HTTP/HTTPS| Proxy
    Proxy -->|MITM TLS| User
    
    Host -->|Docker API| DNS
    Host -->|Manages| IPT
    Host -->|Spawns| Proxy
    Host -->|Spawns| API
    
    API -->|Control| Host
    
    style DNS fill:#4a90e2
    style Proxy fill:#e24a4a
    style API fill:#4ae290
    style Host fill:#e2c44a
```

## Компонентная диаграмма

```mermaid
C4Component
    title Component Diagram - Privacy Hub

    Container_Boundary(host, "Host Process") {
        Component(supervisor, "Supervisor", "Go Module", "Orchestrates lifecycle of all services")
        Component(dockerMgr, "Docker Manager", "Go Module", "Manages DNS container lifecycle")
        Component(iptablesMgr, "IPTables Manager", "Go Module", "Configures network rules")
        Component(apiServer, "API Server", "chi/v5", "REST API for control")
        Component(proxyServer, "Proxy Server", "goproxy", "HTTP/HTTPS MITM proxy")
    }
    
    Container_Boundary(container, "DNS Container") {
        Component(resolver, "DNS Resolver", "miekg/dns", "Handles DNS queries")
        Component(cache, "Cache Layer", "Custom LRU", "Caches DNS responses")
        Component(filter, "Domain Filter", "Custom", "Blocklist/allowlist filtering")
    }
    
    System_Ext(docker, "Docker Engine", "Container runtime")
    System_Ext(kernel, "Linux Kernel", "netfilter/iptables")
    System_Ext(upstream, "Upstream DNS", "DoT/DoH providers")
    
    Rel(supervisor, dockerMgr, "Starts/Stops")
    Rel(supervisor, iptablesMgr, "Configures")
    Rel(supervisor, apiServer, "Spawns goroutine")
    Rel(supervisor, proxyServer, "Spawns goroutine")
    
    Rel(dockerMgr, docker, "Docker API")
    Rel(iptablesMgr, kernel, "iptables commands")
    
    Rel(resolver, cache, "Queries")
    Rel(resolver, filter, "Checks")
    Rel(resolver, upstream, "Forwards queries")
```

## Поток данных DNS-запроса

```mermaid
sequenceDiagram
    actor User
    participant IPT as iptables
    participant DNS as DNS Resolver
    participant Cache
    participant Filter
    participant DoT as Upstream DoT
    participant DoH as Upstream DoH
    
    User->>IPT: DNS Query (port 53)
    IPT->>DNS: Redirect to port 9000
    
    DNS->>Filter: Check domain
    alt Domain blocked
        Filter-->>DNS: NXDOMAIN
        DNS-->>User: Blocked response
    else Domain allowed
        Filter->>Cache: Continue
        Cache->>Cache: Lookup by domain+type
        
        alt Cache hit
            Cache-->>DNS: Cached response
            DNS-->>User: Return cached
        else Cache miss
            Cache->>DoT: Forward query (TLS)
            
            alt DoT success
                DoT-->>Cache: DNS response
            else DoT failed
                Cache->>DoH: Fallback to HTTPS
                DoH-->>Cache: DNS response
            end
            
            Cache->>Cache: Store with TTL
            Cache-->>DNS: Fresh response
            DNS-->>User: Return resolved
        end
    end
```

## Жизненный цикл системы

```mermaid
stateDiagram-v2
    [*] --> Initializing
    
    Initializing --> ConfigLoading: Parse YAML
    ConfigLoading --> ValidationOK: Validate
    ConfigLoading --> [*]: Validation Failed
    
    ValidationOK --> CreatingSupervisor
    CreatingSupervisor --> StartingDNS: Docker Start
    
    StartingDNS --> WaitingDNS: Container Creating
    WaitingDNS --> HealthCheck: Poll Status
    HealthCheck --> WaitingDNS: Not Ready
    HealthCheck --> ConfiguringIPT: Container Ready
    
    ConfiguringIPT --> StartingProxy: iptables Setup
    StartingProxy --> StartingAPI: Goroutine Spawn
    StartingAPI --> Running: All Services Up
    
    Running --> ShuttingDown: SIGINT/SIGTERM
    ShuttingDown --> StoppingServices: Cancel Context
    StoppingServices --> CleaningIPT: Wait Goroutines
    CleaningIPT --> StoppingDNS: Remove Rules
    StoppingDNS --> [*]: Container Stopped
    
    Running --> Running: Process Requests
```

## Управление конкурентностью

```mermaid
graph LR
    subgraph "Main Goroutine"
        Main[main.Start]
    end
    
    subgraph "Supervisor Context"
        Ctx[context.Context]
        Cancel[context.CancelFunc]
    end
    
    subgraph "Worker Goroutines"
        API[API Server<br/>chi router]
        Proxy[Proxy Server<br/>goproxy]
        DNS_UDP[DNS UDP Server]
        DNS_TCP[DNS TCP Server]
    end
    
    subgraph "Background Tasks"
        Cache_Cleanup[Cache Cleanup<br/>ticker: 1min]
        Health[Container Health<br/>ticker: 500ms]
    end
    
    subgraph "Synchronization"
        WG[sync.WaitGroup]
        Mutex[sync.RWMutex]
    end
    
    Main -->|Creates| Ctx
    Main -->|Spawns| API
    Main -->|Spawns| Proxy
    Main -->|Spawns| DNS_UDP
    Main -->|Spawns| DNS_TCP
    
    API -->|Registers| WG
    Proxy -->|Registers| WG
    
    DNS_UDP -->|Spawns| Cache_Cleanup
    Health -->|Polls| DNS_UDP
    
    Ctx -->|Cancels| API
    Ctx -->|Cancels| Proxy
    Ctx -->|Cancels| DNS_UDP
    Ctx -->|Cancels| DNS_TCP
    
    API -->|Protects| Mutex
    Proxy -->|Protects| Mutex
    
    Main -->|Waits| WG
    Cancel -->|Triggers| Ctx
```

## Схема кеширования DNS

```mermaid
graph TD
    subgraph "Cache Structure"
        Map[map string cacheEntry]
        Entry1[Entry:<br/>domain+type<br/>msg: dns.Msg<br/>expiresAt: time.Time]
        Entry2[Entry:<br/>google.com:A<br/>TTL: 300s]
    end
    
    subgraph "Cache Operations"
        Get[Get domain, qtype]
        Set[Set domain, qtype, msg]
        Evict[evictOldest]
        Cleanup[cleanup ticker]
    end
    
    subgraph "Synchronization"
        RWMutex[sync.RWMutex]
    end
    
    Get -->|Read Lock| RWMutex
    Set -->|Write Lock| RWMutex
    Set -->|Size >= maxSize| Evict
    
    Cleanup -->|Every 1 min| Map
    Cleanup -->|Deletes expired| Entry1
    
    Map --> Entry1
    Map --> Entry2
    
    Evict -->|Finds oldest| Map
    Evict -->|Deletes| Entry1
```

## Обработка HTTPS через MITM

```mermaid
sequenceDiagram
    actor Client
    participant Proxy as goproxy
    participant CA as Custom CA
    participant TLS as TLS Handler
    participant Server as Target Server
    
    Client->>Proxy: CONNECT example.com:443
    Proxy->>Proxy: Intercept CONNECT
    
    alt MITM Enabled
        Proxy->>CA: Load CA cert/key
        CA->>TLS: Generate dynamic cert<br/>for example.com
        TLS->>Client: Present fake cert
        Client->>TLS: TLS Handshake
        
        TLS->>Proxy: Decrypt HTTPS
        Proxy->>Proxy: Filter headers<br/>(X-Forwarded-For, Via, etc)
        Proxy->>Proxy: Replace User-Agent
        
        Proxy->>Server: Re-encrypt with real cert
        Server->>Proxy: Response
        Proxy->>TLS: Encrypt for client
        TLS->>Client: Encrypted response
    else MITM Disabled
        Proxy->>Server: Transparent tunnel
        Server-->>Client: Direct encrypted channel
    end
```

## Структура модулей

```mermaid
graph TD
    subgraph "cmd/"
        MainHub[privacy-hub/main.go]
        MainDNS[dnsserver/main.go]
    end
    
    subgraph "internal/"
        Config[config/<br/>config.go<br/>types.go]
        Logger[logger/<br/>logger.go]
        Supervisor[supervisor/<br/>supervisor.go]
        
        subgraph "hubctl/"
            Docker[docker.go]
            IPTables[iptables.go]
        end
        
        subgraph "dnsresolver/"
            Resolver[resolver.go]
            CacheMod[cache.go]
            FilterMod[filter.go]
        end
        
        subgraph "proxyserver/"
            ProxyMod[proxy.go]
        end
        
        subgraph "api/"
            APIMod[api.go]
        end
    end
    
    MainHub --> Supervisor
    MainDNS --> Resolver
    
    Supervisor --> Config
    Supervisor --> Logger
    Supervisor --> Docker
    Supervisor --> IPTables
    Supervisor --> APIMod
    Supervisor --> ProxyMod
    
    Resolver --> CacheMod
    Resolver --> FilterMod
    Resolver --> Config
    Resolver --> Logger
    
    Docker --> Config
    IPTables --> Config
    ProxyMod --> Config
    APIMod --> Config
```

## Детали реализации

### DNS Resolver

**Особенности:**

- Асинхронная обработка UDP/TCP запросов через отдельные горутины
- Поддержка DNS-over-TLS (DoT) с TLS 1.2+ валидацией
- Fallback на DNS-over-HTTPS при недоступности DoT upstream
- Thread-safe LRU кеш с автоматической эвикцией устаревших записей
- Иерархическая фильтрация доменов

**Оптимизации:**

- Переиспользование DNS клиентов через connection pooling
- Минимальный TTL из всех RR для корректного кеширования
- Параллельная обработка запросов без блокировок

### HTTP/HTTPS Proxy

**Возможности:**

- Man-in-the-Middle (MITM) с динамической генерацией сертификатов
- Удаление идентифицирующих заголовков (X-Forwarded-For, Via, Client-IP и др.)
- Подмена User-Agent для единообразия отпечатка браузера
- Обработка CONNECT туннелирования для HTTPS
- Настраиваемые таймауты для предотвращения зависаний

**Безопасность:**

- Использование собственного CA для MITM
- TLS конфигурация с минимальной версией 1.2
- Контролируемый доступ к приватным ключам

### Docker Manager

**Функционал:**

- Программное управление жизненным циклом контейнеров
- Автоматическое создание контейнера при отсутствии
- Health checks с таймаутами для проверки готовности
- Port binding на localhost для изоляции
- Настраиваемые restart policies

**Надежность:**

- Graceful shutdown с таймаутами
- Автоматический rollback при ошибках конфигурации
- Streaming логов из контейнера

### IPTables Manager

**Управление правилами:**

- Динамическое создание NAT rules для перенаправления DNS
- Поддержка UDP (порт 53 → 9000) и TCP (порт 53 → 9001)
- Автоматическая очистка при завершении работы
- Идемпотентность операций

**Безопасность:**

- Минимальный набор правил для снижения attack surface
- Rollback при частичном применении правил
- Thread-safe операции через mutex

### API Server

**Архитектура:**

- RESTful API на основе chi
- Middleware цепочка: RequestID → RealIP → Logging → Recovery → Timeout
- Graceful shutdown с drain периодом
- Structured logging всех HTTP запросов

**Endpoints:**

- `GET /health` — health check для мониторинга
- `GET /config` — просмотр текущей конфигурации
- `POST /restart` — программный перезапуск сервисов

## Конфигурация

Система использует YAML-конфигурацию с валидацией на этапе загрузки:

```yaml
dns:
  listen: ":9000"
  upstreams:
    - "1.1.1.1:853"
    - "8.8.8.8:853"
  doh_upstreams:
    - "https://cloudflare-dns.com/dns-query"
  cache_size: 10000
  cache_ttl: 3600
  enable_filtering: true
```

## Установка и запуск

### Предварительные требования

- Go 1.24+
- Docker и Docker Compose
- Linux с поддержкой iptables
- Права sudo для настройки сетевых правил

### Быстрый старт

```bash
# Сборка всех компонентов
task build

# Генерация MITM сертификатов
task certs:gen

# Установка сертификатов в браузер
task certs:install-firefox # Для firefox

# Запуск полного стека
task up
```

### Проверка работоспособности

```bash
# Проверка всех сервисов
task check

# Тестирование DNS
dig @127.0.0.1 -p 9000 google.com

# Тестирование прокси
curl -x http://localhost:3128 http://example.com

# Проверка API
curl http://localhost:8000/health
```

## Производительность и результаты тестирования

Тестирование проводилось на процессоре AMD Ryzen 7 7840HS в среде Linux.

### Реальные показатели (Benchmarks)

| Метрика | Результат | Примечание |
| :--- | :--- | :--- |
| **Пропускная способность (QPS)** | ~143,000+ запросов/сек | При 100% попадании в кеш (cache hit) |
| **Латентность (cache hit)** | 140.2 ns/op | Измерено через Go Benchmark |
| **Средняя сетевая задержка** | 0.69 ms | С учетом накладных расходов UDP стека |
| **Минимальная задержка** | 27 μs | Лучший показатель при обработке из памяти |
| **Латентность (cache miss, DoT)** | 82 ms | Включая TLS Handshake с upstream сервером |
| **Аллокация памяти (кеш)** | 480 B/запись | 13 аллокаций на одну новую запись в кеше |

### Технический анализ результатов

#### Эффективность кеширования

Результаты `BenchmarkCacheHit` (140.2 нс) подтверждают экстремально низкую нагрузку на CPU при работе с `sync.RWMutex`. Система способна обрабатывать миллионы запросов в секунду внутри процесса, а итоговый сетевой показатель в **143,762 QPS** ограничен лишь скоростью работы сетевого интерфейса и планировщика задач.

#### Сетевая подсистема

Инструмент `dnsperf` зафиксировал 100% успешных ответов (**NOERROR**) при полном отсутствии потерь пакетов на дистанции в 4.3 миллиона запросов, что говорит о высокой стабильности UDP-сервера на базе библиотеки `miekg/dns`.

#### Использование ресурсов

- **Memory Footprint**: При аллокации 480 байт на запись, кеш на 10,000 записей занимает всего около 4.8 MB, что позволяет масштабировать кеш до сотен тысяч записей на стандартном серверном оборудовании.
- **CPU Scalability**: Среднее время выполнения одного запроса через DoT (82 мс) оправдывает использование агрессивного кеширования, так как разница между cache-hit и cache-miss составляет более пяти порядков.

### Воспроизведение тестов

Для получения аналогичных цифр используйте встроенные команды:

```bash
# Локальные бенчмарки
go test -bench=. -benchmem ./internal/dnsresolver

# Нагрузочный тест (требует task up)
dnsperf -s 127.0.0.1 -p 9000 -d test_queries.txt -l 30
```

### Оптимизации

- Zero-copy operations для DNS протокола через `miekg/dns`
- Lock-free reads для кеша через `sync.RWMutex`
- Connection pooling для upstream соединений
- Batch eviction в кеше для снижения накладных расходов

## Лицензия

MIT License
