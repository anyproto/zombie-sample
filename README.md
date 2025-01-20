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
   main_modernc_test.go:74: created 100 tables; 7.533291ms
    main_modernc_test.go:86: prepared 1 read connections
    main_modernc_test.go:131: inserted 3000 rows; 81.042959ms
    main_modernc_test.go:172: read connection 0 processed 10000 queries; 205.214416ms; avg 20.521µs/query
    main_modernc_test.go:117: created 1000 extra tables; 11.791786708s
    main_modernc_test.go:177: Test finished successfully
```

top
```
      flat  flat%   sum%        cum   cum%
     0.27s 23.48% 23.48%      0.27s 23.48%  runtime.pthread_cond_signal
     0.25s 21.74% 45.22%      0.25s 21.74%  syscall.syscall
     0.18s 15.65% 60.87%      0.18s 15.65%  syscall.syscall6
     0.08s  6.96% 67.83%      0.08s  6.96%  runtime.pthread_cond_wait
     0.05s  4.35% 72.17%      0.05s  4.35%  runtime.usleep
     0.04s  3.48% 75.65%      0.04s  3.48%  runtime.kevent
     0.03s  2.61% 78.26%      0.03s  2.61%  modernc.org/sqlite/lib._btreeParseCellPtr
     0.02s  1.74% 80.00%      0.02s  1.74%  runtime.pthread_kill
     0.02s  1.74% 81.74%      0.02s  1.74%  sync.(*Mutex).Lock
     0.01s  0.87% 82.61%      0.01s  0.87%  modernc.org/libc.X__builtin___memcpy_chk
```

## run modernc sample with 16 connections
`go test -v -run SQLiteOperationsModernc16Conn -count 1 -cpuprofile cpu_modernc_16.out && go tool pprof cpu_modernc_16.out`
You'll see:
```
  main_modernc_test.go:74: created 100 tables; 15.366333ms
    main_modernc_test.go:86: prepared 16 read connections
    main_modernc_test.go:131: inserted 3000 rows; 159.670625ms
    main_modernc_test.go:172: read connection 5 processed 10000 queries; 1.606798375s; avg 2.570877ms/query
    main_modernc_test.go:172: read connection 15 processed 10000 queries; 1.678334417s; avg 2.685335ms/query
    main_modernc_test.go:172: read connection 9 processed 10000 queries; 1.701348584s; avg 2.722157ms/query
    main_modernc_test.go:172: read connection 10 processed 10000 queries; 1.70956075s; avg 2.735297ms/query
    main_modernc_test.go:172: read connection 6 processed 10000 queries; 1.715997583s; avg 2.745596ms/query
    main_modernc_test.go:172: read connection 8 processed 10000 queries; 1.717039417s; avg 2.747263ms/query
    main_modernc_test.go:172: read connection 1 processed 10000 queries; 1.721902291s; avg 2.755043ms/query
    main_modernc_test.go:172: read connection 4 processed 10000 queries; 1.725754917s; avg 2.761207ms/query
    main_modernc_test.go:172: read connection 12 processed 10000 queries; 1.733519625s; avg 2.773631ms/query
    main_modernc_test.go:172: read connection 13 processed 10000 queries; 1.736158709s; avg 2.777853ms/query
    main_modernc_test.go:172: read connection 2 processed 10000 queries; 1.7488855s; avg 2.798216ms/query
    main_modernc_test.go:172: read connection 3 processed 10000 queries; 1.750401708s; avg 2.800642ms/query
    main_modernc_test.go:172: read connection 11 processed 10000 queries; 1.751397542s; avg 2.802236ms/query
    main_modernc_test.go:172: read connection 0 processed 10000 queries; 1.756709625s; avg 2.810735ms/query
    main_modernc_test.go:172: read connection 14 processed 10000 queries; 1.757092583s; avg 2.811348ms/query
    main_modernc_test.go:172: read connection 7 processed 10000 queries; 1.759543666s; avg 2.815269ms/query
    main_modernc_test.go:117: created 1000 extra tables; 11.6858805s
    main_modernc_test.go:177: Test finished successfully
```

top

```
Showing top 10 nodes out of 202
      flat  flat%   sum%        cum   cum%
    1470ms 34.83% 34.83%     1470ms 34.83%  runtime.usleep
     710ms 16.82% 51.66%      710ms 16.82%  runtime.pthread_cond_wait
     430ms 10.19% 61.85%      430ms 10.19%  runtime.pthread_cond_signal
     220ms  5.21% 67.06%      220ms  5.21%  syscall.syscall6
     150ms  3.55% 70.62%      150ms  3.55%  runtime.madvise
     130ms  3.08% 73.70%      130ms  3.08%  syscall.syscall
     110ms  2.61% 76.30%      110ms  2.61%  runtime.(*mspan).heapBitsSmallForAddr
      70ms  1.66% 77.96%       70ms  1.66%  runtime.memclrNoHeapPointers
      60ms  1.42% 79.38%       60ms  1.42%  runtime.kevent
      50ms  1.18% 80.57%      210ms  4.98%  runtime.scanobject
```