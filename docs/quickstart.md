# roc_way 快速上手（5 分钟跑通 admin）

## 1. 启动 MySQL & Redis

```
docker compose -f deployments/docker-compose.yml up -d mysql redis
```

## 2. 启动 admin

```
go build -o bin/rocway ./cmd/rocway
./bin/rocway
```

## 3. 验证

```
curl http://localhost:8080/healthz
# {"status":"ok"}

curl -X POST http://localhost:8080/auth/login -d '{"user_id":"alice"}' -H 'Content-Type: application/json'
# {"code":0,"data":{"access":"...","refresh":"...","access_exp":...,"refresh_exp":...}}
```

## 4. 使用 CLI 生成新项目

```
go build -o bin/rocway-cli ./cmd/rocway-cli
./bin/rocway-cli new myapp
cd myapp && go mod init github.com/me/myapp && go run ./cmd/myapp
./bin/rocway-cli gen controller order
```

## 5. 一键 docker-compose 全栈

```
docker compose -f deployments/docker-compose.yml up
```
