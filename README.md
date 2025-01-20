# zombie-sample

The bottleneck appears to be the `libc.Xmalloc` mutex acquisition, as seen in this [code snippet](https://gitlab.com/cznic/libc/-/blob/master/mem.go#L22).

This issue doesn’t occur when using similar code with the vanilla modernc SQLite driver; in that case, the program simply idles.

I prepared two tests to investigate further:
- One using the modernc driver
- Another using the zombie driver

Both tests were run with 1 and 16 read connections. When the driver is heavily stressed by creating tables and inserting data in parallel with reading operations, performance is significantly affected.

Pay close attention to the average query times. While both drivers experience performance degradation under these conditions, the zombie driver suffers more due to the mutex contention in `Xmalloc`.

## run zombie sample with 1 connection
`go test -v -run SQLiteOperationsZombie1Conn -count 1 -cpuprofile cpu_zombie_1.out && go tool pprof cpu_zombie_1.out`
You'll see:
```
    main_zombie_test.go:80: created 100 tables; 31.331958ms
    main_zombie_test.go:91: prepared 1 read connections
    main_zombie_test.go:215: read connection 0 processed 10000 queries; 149.297875ms; avg 14.929µs/query
    main_zombie_test.go:161: inserted 3000 rows; 300.010583ms
    main_zombie_test.go:124: created 1000 extra tables; 12.428120041s
    main_zombie_test.go:220: Test finished successfully
```

top

```
      flat  flat%   sum%        cum   cum%
    81.70s 75.85% 75.85%     81.70s 75.85%  runtime.usleep
     6.95s  6.45% 82.30%      6.95s  6.45%  runtime.pthread_cond_wait
     5.21s  4.84% 87.14%      5.21s  4.84%  runtime.pthread_cond_signal
     2.61s  2.42% 89.56%      2.62s  2.43%  syscall.syscall6
     2.05s  1.90% 91.47%     37.98s 35.26%  sync.(*Mutex).lockSlow
     1.29s  1.20% 92.67%      1.29s  1.20%  runtime.pthread_mutex_lock
     0.80s  0.74% 93.41%     49.10s 45.59%  sync.(*Mutex).Unlock
     0.56s  0.52% 93.93%      0.57s  0.53%  runtime.nanotime1
     0.31s  0.29% 94.22%     78.75s 73.11%  runtime.lock2
     0.26s  0.24% 94.46%     29.10s 27.02%  modernc.org/sqlite/lib._sqlite3VdbeExec

```


## run zombie sample with 16 connections
`go test -v -run SQLiteOperationsZombie16Conn -count 1 -cpuprofile cpu_zombie_16.out && go tool pprof cpu_zombie_16.out`
You'll see:
```
    main_zombie_test.go:80: created 100 tables; 23.957292ms
    main_zombie_test.go:91: prepared 16 read connections
    main_zombie_test.go:124: created 1000 extra tables; 15.130718625s
    main_zombie_test.go:161: inserted 3000 rows; 16.504654125s
    main_zombie_test.go:215: read connection 8 processed 10000 queries; 18.384090208s; avg 29.414544ms/query
    main_zombie_test.go:215: read connection 15 processed 10000 queries; 18.385074417s; avg 29.416119ms/query
    main_zombie_test.go:215: read connection 10 processed 10000 queries; 18.3854345s; avg 29.416695ms/query
    main_zombie_test.go:215: read connection 4 processed 10000 queries; 18.387494541s; avg 29.419991ms/query
    main_zombie_test.go:215: read connection 14 processed 10000 queries; 18.388340917s; avg 29.421345ms/query
    main_zombie_test.go:215: read connection 6 processed 10000 queries; 18.389326s; avg 29.422921ms/query
    main_zombie_test.go:215: read connection 1 processed 10000 queries; 18.392134375s; avg 29.427415ms/query
    main_zombie_test.go:215: read connection 11 processed 10000 queries; 18.395886042s; avg 29.433417ms/query
    main_zombie_test.go:215: read connection 0 processed 10000 queries; 18.396177875s; avg 29.433884ms/query
    main_zombie_test.go:215: read connection 12 processed 10000 queries; 18.39923925s; avg 29.438782ms/query
    main_zombie_test.go:215: read connection 13 processed 10000 queries; 18.399947166s; avg 29.439915ms/query
    main_zombie_test.go:215: read connection 7 processed 10000 queries; 18.401310667s; avg 29.442097ms/query
    main_zombie_test.go:215: read connection 3 processed 10000 queries; 18.4020775s; avg 29.443324ms/query
    main_zombie_test.go:215: read connection 9 processed 10000 queries; 18.402550375s; avg 29.44408ms/query
    main_zombie_test.go:215: read connection 5 processed 10000 queries; 18.402854125s; avg 29.444566ms/query
    main_zombie_test.go:215: read connection 2 processed 10000 queries; 18.403260667s; avg 29.445217ms/query
    main_zombie_test.go:220: Test finished successfully

```

top

```
      flat  flat%   sum%        cum   cum%
    81.94s 75.47% 75.47%     81.94s 75.47%  runtime.usleep
     7.18s  6.61% 82.09%      7.18s  6.61%  runtime.pthread_cond_wait
     5.07s  4.67% 86.76%      5.07s  4.67%  runtime.pthread_cond_signal
     2.45s  2.26% 89.01%      2.45s  2.26%  syscall.syscall6
     2.39s  2.20% 91.21%     38.15s 35.14%  sync.(*Mutex).lockSlow
     1.02s  0.94% 92.15%      1.02s  0.94%  runtime.pthread_mutex_lock
     0.95s  0.88% 93.03%     49.36s 45.46%  sync.(*Mutex).Unlock
     0.38s  0.35% 93.38%     79.02s 72.78%  runtime.lock2
     0.33s   0.3% 93.68%     60.07s 55.33%  modernc.org/sqlite/lib._yy_reduce
     0.30s  0.28% 93.96%     29.88s 27.52%  modernc.org/sqlite/lib._sqlite3VdbeExec

```

## run modernc sample with 1 connections
`go test -v -run SQLiteOperationsModernc1Conn -count 1 -cpuprofile cpu_modernc_1.out && go tool pprof cpu_modernc_1.out`
You'll see:
```
   main_modernc_test.go:59: created 100 tables; 9.249833ms
    main_modernc_test.go:67: prepared 1 read connections
    main_modernc_test.go:100: inserted 3000 rows; 88.906292ms
    main_modernc_test.go:126: read connection 0 processed 10000 queries; 154.517833ms; avg 15.451µs/query
    main_modernc_test.go:86: created 1000 extra tables; 11.71770125s
    main_modernc_test.go:131: Test finished successfully

```

top
```
      flat  flat%   sum%        cum   cum%
Showing top 10 nodes out of 182
      flat  flat%   sum%        cum   cum%
     360ms 38.30% 38.30%      360ms 38.30%  syscall.syscall6
     190ms 20.21% 58.51%      190ms 20.21%  syscall.syscall
     110ms 11.70% 70.21%      110ms 11.70%  runtime.pthread_cond_signal
      80ms  8.51% 78.72%       80ms  8.51%  runtime.pthread_cond_wait
      40ms  4.26% 82.98%       40ms  4.26%  modernc.org/sqlite/lib._btreeParseCellPtr
      20ms  2.13% 85.11%       20ms  2.13%  modernc.org/sqlite/lib._sqlite3MemMalloc
      20ms  2.13% 87.23%      690ms 73.40%  modernc.org/sqlite/lib._sqlite3VdbeExec
      10ms  1.06% 88.30%       10ms  1.06%  modernc.org/libc.Xmemset
      10ms  1.06% 89.36%       10ms  1.06%  modernc.org/sqlite/lib.Xsqlite3_mutex_try
      10ms  1.06% 90.43%       20ms  2.13%  modernc.org/sqlite/lib._btreeNext

```

## run modernc sample with 16 connections
`go test -v -run SQLiteOperationsModernc16Conn -count 1 -cpuprofile cpu_modernc_16.out && go tool pprof cpu_modernc_16.out`
You'll see:
```
     main_modernc_test.go:126: read connection 15 processed 10000 queries; 1.50213225s; avg 2.403411ms/query
    main_modernc_test.go:126: read connection 7 processed 10000 queries; 1.511492041s; avg 2.418387ms/query
    main_modernc_test.go:126: read connection 4 processed 10000 queries; 1.51270775s; avg 2.420332ms/query
    main_modernc_test.go:126: read connection 10 processed 10000 queries; 1.513714042s; avg 2.421942ms/query
    main_modernc_test.go:126: read connection 12 processed 10000 queries; 1.51636225s; avg 2.426179ms/query
    main_modernc_test.go:126: read connection 1 processed 10000 queries; 1.516493916s; avg 2.42639ms/query
    main_modernc_test.go:126: read connection 8 processed 10000 queries; 1.516885708s; avg 2.427017ms/query
    main_modernc_test.go:126: read connection 5 processed 10000 queries; 1.517493375s; avg 2.427989ms/query
    main_modernc_test.go:126: read connection 3 processed 10000 queries; 1.51758025s; avg 2.428128ms/query
    main_modernc_test.go:86: created 1000 extra tables; 11.600858833s
    main_modernc_test.go:131: Test finished successfully

```

top

```
Showing top 10 nodes out of 153
      flat  flat%   sum%        cum   cum%
     0.93s 34.70% 34.70%      0.93s 34.70%  syscall.syscall
     0.56s 20.90% 55.60%      0.56s 20.90%  syscall.syscall6
     0.43s 16.04% 71.64%      0.43s 16.04%  runtime.pthread_cond_wait
     0.25s  9.33% 80.97%      0.25s  9.33%  runtime.pthread_cond_signal
     0.21s  7.84% 88.81%      0.21s  7.84%  runtime.usleep
     0.04s  1.49% 90.30%      1.60s 59.70%  modernc.org/sqlite/lib._sqlite3VdbeExec
     0.03s  1.12% 91.42%      0.03s  1.12%  modernc.org/sqlite/lib._btreeParseCellPtr
     0.02s  0.75% 92.16%      0.02s  0.75%  modernc.org/sqlite/lib._sqlite3FkClearTriggerCache
     0.02s  0.75% 92.91%      0.07s  2.61%  modernc.org/sqlite/lib._yy_reduce
     0.02s  0.75% 93.66%      0.02s  0.75%  runtime.kevent

```