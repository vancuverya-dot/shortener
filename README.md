# Профилирование и оптимизация производительности

## Что было найдено

Профиль снимался под нагрузкой `hey` (3000 запросов, 30 конкурентных соединений) на эндпоинты `POST /` и `GET /{id}` с реальной PostgreSQL.

Анализ `go tool pprof -alloc_space` базового профиля показал две проблемы:

**1. `GzipMiddleware` — 95.57% всех аллокаций памяти**

`compress/flate.NewWriter` и `compress/flate.(*compressor).initDeflate` суммарно занимали почти весь объём аллокаций. Причина: `GzipMiddleware` создавала новый `gzip.Writer` на каждый HTTP-запрос. `gzip.NewWriter` аллоцирует внутренние буферы сжатия (LZ77-окно, таблицы Хаффмана) — дорогая операция при высокой частоте запросов.

**Исправление:** `sync.Pool` для `*gzip.Writer` и `*gzip.Reader` — объекты переиспользуются через `Reset()`, буферы не пересоздаются (`cmd/shortener/compress.go`).

**2. `storage.InsertURL` — 27.96% всех аллокаций объектов**

Функция выполняла два последовательных запроса к БД на каждую вставку: сначала `UPDATE ... RETURNING` (попытка восстановить удалённый URL), и только при неудаче — `INSERT`. В обычном случае (новый URL) первый запрос был всегда лишним, что удваивало нагрузку на сетевой протокол PostgreSQL (буферы, контекст-вотчер, парсинг ответа).

**Исправление:** единый `INSERT ... ON CONFLICT (urls_original_url) DO UPDATE ... RETURNING urls_short_url, (xmax = 0) AS inserted` — один запрос покрывает все случаи. Системный столбец `xmax = 0` позволяет отличить вставку от обновления без дополнительного запроса (`internal/storage/urlsq.go`).

## Результат

```
go tool pprof -top -alloc_space -diff_base=profiles/base.pprof profiles/result.pprof
```

```
Showing nodes accounting for -2362.44MB, 95.97% of 2461.73MB total
      flat  flat%   sum%        cum   cum%
-1907.42MB 77.48% 77.48% -2334.37MB 94.83%  compress/flate.NewWriter (inline)
 -414.94MB 16.86% 94.34%  -414.94MB 16.86%  compress/flate.(*compressor).initDeflate (inline)
  -22.55MB  0.92% 95.25%   -22.55MB  0.92%  compress/flate.(*huffmanEncoder).generate
  -15.03MB  0.61% 95.87%   -15.03MB  0.61%  sync.(*Pool).pinSlow
      -3MB  0.12% 95.99% -2396.94MB 97.37%  main.GzipMiddleware.func1
    0.50MB  0.02% 95.97%   -35.53MB  1.44%  main.Logging.func1
```

```
go tool pprof -top -alloc_objects -diff_base=profiles/base.pprof profiles/result.pprof
```

```
Showing nodes accounting for -471599, 43.93% of 1073590 total
Dropped 51 nodes (cum <= 5367)
      flat  flat%   sum%        cum   cum%
    -65538  6.10%  6.10%    -254177 23.68%  main.GzipMiddleware.func1
     65537  6.10% 9.3e-05%      65537  6.10%  reflect.unsafe_New
    -49152  4.58%  4.58%    -145708 13.57%  github.com/vancuverya-dot/shortener/internal/storage.InsertURL
    -42302  3.94%  8.52%     -42302  3.94%  net/textproto.readMIMEHeader
    -40217  3.75% 12.26%     -40217  3.75%  net/textproto.MIMEHeader.Set (inline)
    -32769  3.05% 15.32%     -32769  3.05%  container/list.(*List).insertValue (inline)
    -32768  3.05% 18.37%     -32768  3.05%  github.com/jackc/pgx/v5.(*baseRows).Scan
     32768  3.05% 15.32%      32768  3.05%  github.com/jackc/pgx/v5/pgconn.(*PgConn).makeCommandTag (inline)
    -32768  3.05% 18.37%     119712 11.15%  github.com/vancuverya-dot/shortener/internal/auth.genToken
     32768  3.05% 15.32%      32768  3.05%  internal/strconv.FormatInt
    -32768  3.05% 21.42%     -32768  3.05%  net.IP.String
    -32768  3.05% 21.42%     -32768  3.05%  net/http.(*connReader).startBackgroundRead
     32768  3.05% 18.37%      24576  2.29%  net/textproto.(*Reader).ReadLine (inline)
     31209  2.91% 15.46%      31209  2.91%  encoding/base64.(*Encoding).EncodeToString
    -30428  2.83% 18.30%     -42312  3.94%  github.com/jackc/pgx/v5/pgconn/ctxwatch.(*ContextWatcher).Watch
    -27308  2.54% 20.84%     -27308  2.54%  sync.(*poolChain).pushHead
    -26216  2.44% 23.28%     -43285  4.03%  net.(*conn).Read
    -22983  2.14% 25.42%     -22983  2.14%  compress/flate.newHuffmanEncoder (inline)
    -22021  2.05% 27.47%     -22021  2.05%  go.uber.org/zap/internal/bufferpool.init.NewPool.func1
     21845  2.03% 25.44%      18866  1.76%  github.com/sixafter/prng-chacha.newCipher
     21845  2.03% 23.40%    -129821 12.09%  main.Logging.func1
     16384  1.53% 21.88%      16384  1.53%  crypto/internal/fips140/sha256.(*Digest).Sum
    -16384  1.53% 23.40%     -16384  1.53%  github.com/jackc/puddle/v2.newValueCancelCtx (inline)
    -16384  1.53% 24.93%     -30414  2.83%  github.com/vancuverya-dot/shortener/internal/storage.GetOriginalURL
    -16384  1.53% 26.46%     -16384  1.53%  go.uber.org/zap/buffer.(*Buffer).String (inline)
    -13108  1.22% 27.68%     -13108  1.22%  context.withCancel (inline)
    -13108  1.22% 28.90%     -13108  1.22%  github.com/jackc/pgx/v5/pgconn.(*ResultReader).Read
    -11838  1.10% 30.00%     -34821  3.24%  compress/flate.newHuffmanBitWriter (inline)
    -11473  1.07% 31.07%     -44447  4.14%  net/http.readRequest
    -11265  1.05% 32.12%     -11265  1.05%  go.uber.org/zap/internal/stacktrace.init.func1
    -10923  1.02% 33.14%     -10923  1.02%  context.WithValue
    -10923  1.02% 34.15%     -62978  5.87%  net/http.(*conn).readRequest
    -10923  1.02% 35.17%     -10923  1.02%  net/textproto.NewReader (inline)
    -10261  0.96% 36.13%     -10261  0.96%  compress/flate.(*huffmanEncoder).generate
      9832  0.92% 35.21%      26216  2.44%  github.com/golang-jwt/jwt/v5.(*SigningMethodHMAC).Sign
     -9474  0.88% 36.09%      -9474  0.88%  bufio.NewWriterSize (inline)
     -9363  0.87% 36.97%      -9363  0.87%  context.(*cancelCtx).Done
     -9363  0.87% 37.84%     -11884  1.11%  context.AfterFunc
      9363  0.87% 36.97%       9363  0.87%  golang.org/x/sync/semaphore.(*Weighted).Acquire
     -9102  0.85% 37.81%      -9102  0.85%  bytes.(*Buffer).ReadBytes (inline)
      9102  0.85% 36.97%       9102  0.85%  github.com/golang-jwt/jwt/v5.NewWithClaims (inline)
      8192  0.76% 36.20%       8192  0.76%  bytes.(*Buffer).grow
      8192  0.76% 35.44%       8192  0.76%  fmt.(*buffer).writeString (inline)
     -8192  0.76% 36.20%      -8192  0.76%  internal/poll.(*FD).pin
      8192  0.76% 35.44%       8192  0.76%  syscall.(*RawSockaddrAny).Sockaddr
     -7282  0.68% 36.12%      -7282  0.68%  net/url.parse
     -7281  0.68% 36.80%      -7281  0.68%  github.com/jackc/pgx/v5.(*Conn).getRows
     -6840  0.64% 37.43%      -6840  0.64%  sync.(*Pool).pinSlow
     -6554  0.61% 38.04%      58983  5.49%  encoding/json.mapEncoder.encode
     -6554  0.61% 38.65%      12312  1.15%  github.com/sixafter/prng-chacha.newPRNG
     -6253  0.58% 39.24%      -6253  0.58%  compress/flate.(*compressor).initDeflate (inline)
     -6145  0.57% 39.81%      -6145  0.57%  github.com/jackc/pgx/v5/pgconn.ErrorResponseToPgError (inline)
     -5462  0.51% 40.32%      -5462  0.51%  net/http.(*Request).SetPathValue (inline)
     -5461  0.51% 40.83%      -5461  0.51%  github.com/jackc/pgx/v5/internal/stmtcache.StatementName
     -4916  0.46% 41.28%      -4916  0.46%  net/http.(*Request).WithContext (inline)
      4681  0.44% 40.85%       4681  0.44%  github.com/vancuverya-dot/shortener/internal/service.(*Worker).run
     -4681  0.44% 41.28%      -4681  0.44%  time.newTimer
     -4470  0.42% 41.70%      -4470  0.42%  net/textproto.MIMEHeader.Add (inline)
     -4469  0.42% 42.12%      -4469  0.42%  net/http.Header.Clone (inline)
     -4370  0.41% 42.52%     -16319  1.52%  go.uber.org/zap/internal/stacktrace.Capture
     -4233  0.39% 42.92%      -4233  0.39%  unicode/utf16.Encode
     -4098  0.38% 43.30%      -4098  0.38%  io.ReadAll
      3641  0.34% 42.96%       3641  0.34%  encoding/json.Unmarshal
     -3641  0.34% 43.30%     -12743  1.19%  github.com/jackc/pgx/v5/pgproto3.(*ErrorResponse).Decode
     -3277  0.31% 43.61%      97837  9.11%  github.com/golang-jwt/jwt/v5.(*Token).SigningString
     -3014  0.28% 43.89%     -44088  4.11%  compress/flate.NewWriter (inline)
     -2979  0.28% 44.16%      -2979  0.28%  golang.org/x/crypto/chacha20.NewUnauthenticatedCipher (inline)
     -2521  0.23% 44.40%     143378 13.36%  github.com/golang-jwt/jwt/v5.(*Token).SignedString
      2521  0.23% 44.16%       2521  0.23%  github.com/sixafter/nanoid.NewGenerator.func1
     -2521  0.23% 44.40%      -2521  0.23%  go.uber.org/zap/zapcore.init.func2
      2521  0.23% 44.16%       2521  0.23%  net.(*Resolver).lookupIP.func1
      2341  0.22% 43.95%       2341  0.22%  net.newFD (inline)
       195 0.018% 43.93%       8387  0.78%  github.com/jackc/pgx/v5/pgxpool.NewWithConfig.func3
         0     0% 43.93%      -8192  0.76%  bufio.(*Reader).ReadLine
         0     0% 43.93%      -8192  0.76%  bufio.(*Reader).ReadSlice
         0     0% 43.93%      -8192  0.76%  bufio.(*Reader).fill
         0     0% 43.93%       8192  0.76%  bytes.(*Buffer).Write
         0     0% 43.93%     -10261  0.96%  compress/flate.(*Writer).Close (inline)
         0     0% 43.93%     -10261  0.96%  compress/flate.(*compressor).close
         0     0% 43.93%     -10261  0.96%  compress/flate.(*compressor).deflate
         0     0% 43.93%     -41074  3.83%  compress/flate.(*compressor).init
         0     0% 43.93%     -10261  0.96%  compress/flate.(*compressor).writeBlock
         0     0% 43.93%      -7069  0.66%  compress/flate.(*huffmanBitWriter).indexTokens
         0     0% 43.93%     -10261  0.96%  compress/flate.(*huffmanBitWriter).writeBlock
         0     0% 43.93%     -10261  0.96%  compress/gzip.(*Writer).Close
         0     0% 43.93%     -44088  4.11%  compress/gzip.(*Writer).Write
         0     0% 43.93%     -32769  3.05%  container/list.(*List).PushBack (inline)
         0     0% 43.93%      -9363  0.87%  context.(*cancelCtx).propagateCancel
         0     0% 43.93%     -13108  1.22%  context.WithCancel
         0     0% 43.93%      16384  1.53%  crypto/internal/fips140/hmac.(*HMAC).Sum
         0     0% 43.93%       3641  0.34%  encoding/json.(*decodeState).literalStore
         0     0% 43.93%       3641  0.34%  encoding/json.(*decodeState).object
         0     0% 43.93%       3641  0.34%  encoding/json.(*decodeState).unmarshal
         0     0% 43.93%       3641  0.34%  encoding/json.(*decodeState).value
         0     0% 43.93%      91751  8.55%  encoding/json.(*encodeState).marshal
         0     0% 43.93%      91751  8.55%  encoding/json.(*encodeState).reflectValue
         0     0% 43.93%      91751  8.55%  encoding/json.Marshal
         0     0% 43.93%      32768  3.05%  encoding/json.marshalerEncoder
         0     0% 43.93%      32768  3.05%  encoding/json.structEncoder.encode
         0     0% 43.93%       8192  0.76%  fmt.(*fmt).fmtS
         0     0% 43.93%       8192  0.76%  fmt.(*fmt).padString
         0     0% 43.93%       8192  0.76%  fmt.(*pp).doPrintln
         0     0% 43.93%       8192  0.76%  fmt.(*pp).fmtString
         0     0% 43.93%       8192  0.76%  fmt.(*pp).handleMethods
         0     0% 43.93%       8192  0.76%  fmt.(*pp).printArg
         0     0% 43.93%       8192  0.76%  fmt.Sprintln
         0     0% 43.93%    -270928 25.24%  github.com/go-chi/chi/v5.(*Mux).ServeHTTP
         0     0% 43.93%     -89048  8.29%  github.com/go-chi/chi/v5.(*Mux).routeHTTP
         0     0% 43.93%       3641  0.34%  github.com/golang-jwt/jwt/v5.(*NumericDate).UnmarshalJSON
         0     0% 43.93%       3641  0.34%  github.com/golang-jwt/jwt/v5.(*Parser).ParseUnverified
         0     0% 43.93%       3641  0.34%  github.com/golang-jwt/jwt/v5.(*Parser).ParseWithClaims
         0     0% 43.93%      31209  2.91%  github.com/golang-jwt/jwt/v5.(*Token).EncodeSegment (inline)
         0     0% 43.93%      32768  3.05%  github.com/golang-jwt/jwt/v5.NumericDate.MarshalJSON
         0     0% 43.93%       3641  0.34%  github.com/golang-jwt/jwt/v5.ParseWithClaims
         0     0% 43.93%     -63203  5.89%  github.com/jackc/pgx/v5.(*Conn).Exec
         0     0% 43.93%     -21065  1.96%  github.com/jackc/pgx/v5.(*Conn).Prepare
         0     0% 43.93%     -56844  5.29%  github.com/jackc/pgx/v5.(*Conn).Query
         0     0% 43.93%     -56844  5.29%  github.com/jackc/pgx/v5.(*Conn).QueryRow (inline)
         0     0% 43.93%     -32769  3.05%  github.com/jackc/pgx/v5.(*Conn).deallocateInvalidatedCachedStatements
         0     0% 43.93%     -63203  5.89%  github.com/jackc/pgx/v5.(*Conn).exec
         0     0% 43.93%     -36677  3.42%  github.com/jackc/pgx/v5.(*Conn).execPrepared
         0     0% 43.93%      32768  3.05%  github.com/jackc/pgx/v5.(*baseRows).Close
         0     0% 43.93%       8192  0.76%  github.com/jackc/pgx/v5.ConnectConfig
         0     0% 43.93%       8192  0.76%  github.com/jackc/pgx/v5.connect
         0     0% 43.93%     -40363  3.76%  github.com/jackc/pgx/v5/pgconn.(*PgConn).ExecStatement
         0     0% 43.93%     -21065  1.96%  github.com/jackc/pgx/v5/pgconn.(*PgConn).Prepare
         0     0% 43.93%     -21247  1.98%  github.com/jackc/pgx/v5/pgconn.(*PgConn).execExtendedPrefix
         0     0% 43.93%     -19116  1.78%  github.com/jackc/pgx/v5/pgconn.(*PgConn).execExtendedSuffix
         0     0% 43.93%     -12971  1.21%  github.com/jackc/pgx/v5/pgconn.(*PgConn).peekMessage
         0     0% 43.93%     -17068  1.59%  github.com/jackc/pgx/v5/pgconn.(*PgConn).receiveMessage
         0     0% 43.93%     -10923  1.02%  github.com/jackc/pgx/v5/pgconn.(*Pipeline).SendDeallocate
         0     0% 43.93%     -21846  2.03%  github.com/jackc/pgx/v5/pgconn.(*Pipeline).SendPipelineSync
         0     0% 43.93%     -21846  2.03%  github.com/jackc/pgx/v5/pgconn.(*Pipeline).Sync
         0     0% 43.93%      32768  3.05%  github.com/jackc/pgx/v5/pgconn.(*ResultReader).Close
         0     0% 43.93%     -19116  1.78%  github.com/jackc/pgx/v5/pgconn.(*ResultReader).readUntilRowDescription
         0     0% 43.93%      13652  1.27%  github.com/jackc/pgx/v5/pgconn.(*ResultReader).receiveMessage
         0     0% 43.93%     -32769  3.05%  github.com/jackc/pgx/v5/pgconn.(*pipelineState).PushBackRequestType
         0     0% 43.93%       8192  0.76%  github.com/jackc/pgx/v5/pgconn.ConnectConfig
         0     0% 43.93%       8192  0.76%  github.com/jackc/pgx/v5/pgconn.connectOne
         0     0% 43.93%       8192  0.76%  github.com/jackc/pgx/v5/pgconn.connectPreferred
         0     0% 43.93%     -12971  1.21%  github.com/jackc/pgx/v5/pgproto3.(*Frontend).Receive
         0     0% 43.93%     -63203  5.89%  github.com/jackc/pgx/v5/pgxpool.(*Conn).Exec
         0     0% 43.93%     -56844  5.29%  github.com/jackc/pgx/v5/pgxpool.(*Conn).QueryRow
         0     0% 43.93%       9363  0.87%  github.com/jackc/pgx/v5/pgxpool.(*Pool).Acquire
         0     0% 43.93%     -63203  5.89%  github.com/jackc/pgx/v5/pgxpool.(*Pool).Exec
         0     0% 43.93%     -47383  4.41%  github.com/jackc/pgx/v5/pgxpool.(*Pool).QueryRow
         0     0% 43.93%      -6554  0.61%  github.com/jackc/pgx/v5/pgxpool.(*Pool).createIdleResources
         0     0% 43.93%      -6554  0.61%  github.com/jackc/pgx/v5/pgxpool.NewWithConfig.func5
         0     0% 43.93%       9363  0.87%  github.com/jackc/puddle/v2.(*Pool[go.shape.*uint8]).Acquire
         0     0% 43.93%       9363  0.87%  github.com/jackc/puddle/v2.(*Pool[go.shape.*uint8]).acquire
         0     0% 43.93%      -7997  0.74%  github.com/jackc/puddle/v2.(*Pool[go.shape.*uint8]).initResourceValue.func1
         0     0% 43.93%      13693  1.28%  github.com/sixafter/nanoid.(*generator).New
         0     0% 43.93%      13693  1.28%  github.com/sixafter/nanoid.(*generator).NewWithLength
         0     0% 43.93%      13693  1.28%  github.com/sixafter/nanoid.(*generator).newASCII
         0     0% 43.93%      11172  1.04%  github.com/sixafter/prng-chacha.(*reader).Read
         0     0% 43.93%      12312  1.15%  github.com/sixafter/prng-chacha.init.0.func1
         0     0% 43.93%     118883 11.07%  github.com/vancuverya-dot/shortener/internal/auth.GetOrCreateUserID
         0     0% 43.93%       3641  0.34%  github.com/vancuverya-dot/shortener/internal/auth.parseToken
         0     0% 43.93%     -84090  7.83%  github.com/vancuverya-dot/shortener/internal/handler.UrlGet
         0     0% 43.93%      -3137  0.29%  github.com/vancuverya-dot/shortener/internal/handler.UrlPost
         0     0% 43.93%       3641  0.34%  github.com/vancuverya-dot/shortener/internal/handler.UrlPostJson
         0     0% 43.93%     -76720  7.15%  github.com/vancuverya-dot/shortener/internal/storage.GetShortURL
         0     0% 43.93%     -19296  1.80%  go.uber.org/zap.(*Logger).Check (inline)
         0     0% 43.93%     -19296  1.80%  go.uber.org/zap.(*Logger).check
         0     0% 43.93%     -62618  5.83%  go.uber.org/zap.(*SugaredLogger).Infoln
         0     0% 43.93%     -62618  5.83%  go.uber.org/zap.(*SugaredLogger).logln
         0     0% 43.93%       8192  0.76%  go.uber.org/zap.getMessageln (inline)
         0     0% 43.93%      -4096  0.38%  go.uber.org/zap/buffer.(*Buffer).Free (inline)
         0     0% 43.93%     -22477  2.09%  go.uber.org/zap/buffer.Pool.Get
         0     0% 43.93%      -4096  0.38%  go.uber.org/zap/buffer.Pool.put (inline)
         0     0% 43.93%     -22021  2.05%  go.uber.org/zap/internal/bufferpool.init.NewPool.New[go.shape.*uint8].func2
         0     0% 43.93%     -37403  3.48%  go.uber.org/zap/internal/pool.(*Pool[go.shape.*uint8]).Get (inline)
         0     0% 43.93%      -8420  0.78%  go.uber.org/zap/internal/pool.(*Pool[go.shape.*uint8]).Put (inline)
         0     0% 43.93%     -11265  1.05%  go.uber.org/zap/internal/stacktrace.init.New[go.shape.*uint8].func2
         0     0% 43.93%      -2977  0.28%  go.uber.org/zap/zapcore.(*CheckedEntry).AddCore (inline)
         0     0% 43.93%     -51514  4.80%  go.uber.org/zap/zapcore.(*CheckedEntry).Write
         0     0% 43.93%      -2977  0.28%  go.uber.org/zap/zapcore.(*ioCore).Check
         0     0% 43.93%     -47190  4.40%  go.uber.org/zap/zapcore.(*ioCore).Write
         0     0% 43.93%      -4233  0.39%  go.uber.org/zap/zapcore.(*lockedWriteSyncer).Write
         0     0% 43.93%      32768  3.05%  go.uber.org/zap/zapcore.CapitalLevelEncoder
         0     0% 43.93%     -37893  3.53%  go.uber.org/zap/zapcore.EntryCaller.TrimmedPath
         0     0% 43.93%     -70661  6.58%  go.uber.org/zap/zapcore.ShortCallerEncoder
         0     0% 43.93%     -38861  3.62%  go.uber.org/zap/zapcore.consoleEncoder.EncodeEntry
         0     0% 43.93%      -2977  0.28%  go.uber.org/zap/zapcore.getCheckedEntry
         0     0% 43.93%      -2521  0.23%  go.uber.org/zap/zapcore.init.New[go.shape.*uint8].func6
         0     0% 43.93%      -4324   0.4%  go.uber.org/zap/zapcore.putCheckedEntry (inline)
         0     0% 43.93%     -17069  1.59%  internal/poll.(*FD).Read
         0     0% 43.93%      -4461  0.42%  internal/poll.(*FD).Write
         0     0% 43.93%      -9105  0.85%  internal/poll.(*FD).execIO
         0     0% 43.93%      -4233  0.39%  internal/poll.(*FD).writeConsole
         0     0% 43.93%       8192  0.76%  main.(*loggingResponseWriter).Write
         0     0% 43.93%       8192  0.76%  main.(*responseWriter).Write
         0     0% 43.93%      -6554  0.61%  main.main.func1
         0     0% 43.93%       8895  0.83%  main.main.func3
         0     0% 43.93%       8192  0.76%  net.(*Dialer).DialContext
         0     0% 43.93%       2521  0.23%  net.(*Resolver).lookupIP.func2
         0     0% 43.93%     -32768  3.05%  net.(*TCPAddr).String
         0     0% 43.93%       2341  0.22%  net.(*TCPListener).Accept
         0     0% 43.93%       2341  0.22%  net.(*TCPListener).accept
         0     0% 43.93%     -17069  1.59%  net.(*netFD).Read
         0     0% 43.93%       2341  0.22%  net.(*netFD).accept
         0     0% 43.93%       8192  0.76%  net.(*netFD).dial
         0     0% 43.93%       8192  0.76%  net.(*sysDialer).dialParallel
         0     0% 43.93%       8192  0.76%  net.(*sysDialer).dialSerial
         0     0% 43.93%       8192  0.76%  net.(*sysDialer).dialSingle
         0     0% 43.93%       8192  0.76%  net.(*sysDialer).dialTCP
         0     0% 43.93%       8192  0.76%  net.(*sysDialer).doDialTCP (inline)
         0     0% 43.93%       8192  0.76%  net.(*sysDialer).doDialTCPProto
         0     0% 43.93%       8192  0.76%  net.internetSocket
         0     0% 43.93%     -32768  3.05%  net.ipEmptyString (inline)
         0     0% 43.93%       8192  0.76%  net.socket
         0     0% 43.93%       2341  0.22%  net/http.(*Server).ListenAndServe
         0     0% 43.93%       2341  0.22%  net/http.(*Server).Serve
         0     0% 43.93%    -419925 39.11%  net/http.(*conn).serve
         0     0% 43.93%      -8192  0.76%  net/http.(*connReader).Read
         0     0% 43.93%     -34865  3.25%  net/http.(*connReader).backgroundRead
         0     0% 43.93%      -4469  0.42%  net/http.(*response).WriteHeader
         0     0% 43.93%     -12291  1.14%  net/http.(*response).finishRequest
         0     0% 43.93%    -254177 23.68%  net/http.HandlerFunc.ServeHTTP
         0     0% 43.93%      -4470  0.42%  net/http.Header.Add (inline)
         0     0% 43.93%     -40217  3.75%  net/http.Header.Set (inline)
         0     0% 43.93%      -6554  0.61%  net/http.ListenAndServe (inline)
         0     0% 43.93%     -44314  4.13%  net/http.Redirect
         0     0% 43.93%      -4470  0.42%  net/http.SetCookie
         0     0% 43.93%      -9246  0.86%  net/http.newBufioWriterSize
         0     0% 43.93%     -11607  1.08%  net/http.newTextprotoReader
         0     0% 43.93%     -11607  1.08%  net/http.putBufioWriter
         0     0% 43.93%    -270928 25.24%  net/http.serverHandler.ServeHTTP
         0     0% 43.93%     -42302  3.94%  net/textproto.(*Reader).ReadMIMEHeader (inline)
         0     0% 43.93%      -8192  0.76%  net/textproto.(*Reader).readLineSlice
         0     0% 43.93%      -3641  0.34%  net/url.Parse
         0     0% 43.93%      -3641  0.34%  net/url.ParseRequestURI
         0     0% 43.93%      -4233  0.39%  os.(*File).Write
         0     0% 43.93%      -4233  0.39%  os.(*File).write (inline)
         0     0% 43.93%      65537  6.10%  reflect.(*MapIter).Value
         0     0% 43.93%      65537  6.10%  reflect.copyVal
         0     0% 43.93%      32768  3.05%  strconv.FormatInt (inline)
         0     0% 43.93%     -25762  2.40%  sync.(*Pool).Get
         0     0% 43.93%     -29360  2.73%  sync.(*Pool).Put
         0     0% 43.93%      -6840  0.64%  sync.(*Pool).pin
         0     0% 43.93%       8192  0.76%  syscall.Getsockname
         0     0% 43.93%      -4681  0.44%  time.AfterFunc
```