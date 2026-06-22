# roc_way 架构

## 模块依赖

```
cmd/rocway (main)
    └── internal/wire          (编译期 DI)
            └── internal/app/admin
                    ├── internal/pkg/middleware
                    ├── internal/pkg/auth (JWT + Casbin Enforcer)
                    ├── internal/pkg/cache (Redis)
                    ├── internal/pkg/database (GORM)
                    ├── internal/pkg/logger (Zap+Lumberjack)
                    └── internal/pkg/config (Viper)
                              ↑
cmd/rocway-cli ────────────────┘ (只读 assets/scaffold/)
```

## 设计原则

- **配置**：统一使用 viper（YAML + env + 热更新）
- **日志**：Zap + Lumberjack（结构化输出 + 文件轮转）
- **权限**：Casbin（model + policy 解耦，可热更新）
- **依赖注入**：Wire（编译期生成，零反射）
- **CLI**：cobra 顶层 + 标准库 flag 简单子命令
- **存储 / 缓存 / ORM / JWT**：均优先复用高 star 成熟开源包
