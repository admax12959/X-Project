# X-Project

应用维护工具（Application maintenance tool）. Build, run and test: [docs/dev.md](docs/dev.md).

```sh
docker build -t x-project .   # one image: Go server + built frontend
docker-compose up --build     # app + MySQL 8 on http://localhost:8080 (admin / change-me-now)
make check                    # all tests, typecheck, i18n check, frontend build
```
