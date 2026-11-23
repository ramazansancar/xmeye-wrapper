# xmeye-wrapper köprü sunucu — çok aşamalı Go build, küçük runtime imajı.
FROM golang:1.23-alpine AS build
WORKDIR /src
# bağımlılıkları önce (katman önbelleği)
COPY core-go/go.mod ./core-go/go.mod
RUN cd core-go && go mod download || true
COPY core-go/ ./core-go/
RUN cd core-go && CGO_ENABLED=0 go build -ldflags="-s -w" -o /out/xmeye-wrapper ./server

# --- runtime ---
FROM alpine:3.20
RUN adduser -D -u 10001 app
COPY --from=build /out/xmeye-wrapper /usr/local/bin/xmeye-wrapper
USER app
EXPOSE 8080
ENV XMEYE_ADDR=:8080
ENTRYPOINT ["/usr/local/bin/xmeye-wrapper"]
