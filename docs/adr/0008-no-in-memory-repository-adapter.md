# 不为 Repository 建内存 adapter，加锁与事务行为用 MySQL 容器测试

领域层的 `Repository` 只有 MySQL 一个 adapter。我们否决了再写一个内存 adapter 来缩小测试面（包括拆分 `Repository` 的 sub-interface 来配合它）：内存实现复现不了 `FOR UPDATE` 的行锁与加锁顺序，也复现不了事务被中止后的行为（`IsTxAborting`），而第二期认证加固里的 bug（加锁顺序、在锁内复查、通知错误是否让事务继续存活）全出在这两处，内存 fake 只会让这类测试在错误的地方通过。需要断言调用顺序时，用 `internal/fakes/` 里一份薄的录制式 fake；需要验证真实锁行为时，用 `test/api` 的 MySQL 8 容器。

## Consequences

- `Repository` 保持一个 adapter，测试用 MySQL 容器覆盖真实行为，录制 fake 只断言顺序。
- 各测试文件里各自内嵌 nil `Repository` 的 fake，合并为 `internal/fakes/` 里的一份共用录制 fake。
- 想再提「内存 Repository」或「按 feature 拆 `Repository`」的架构审查，需要先说明如何复现上述两类行为。
