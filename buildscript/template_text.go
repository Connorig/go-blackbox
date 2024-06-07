package buildscript

const script = `#!/bin/bash
Name="{{ .Name }}"
MainPath="{{ .Main }}"
Org="{{ .Org }}"

# shellcheck disable=SC2046
Version=$(git describe --tags $(git rev-list --tags --max-count=1))
# shellcheck disable=SC2154
GitCommit=$(git log --pretty=format:"%h" -1)
BuildTime=$(date +%FT%T%z)

build_image(){
  git checkout "${Version}"
  docker build -t "${Org}/${Name}:${Version}" \
  --build-arg NAME="${Name}" \
  --build-arg VERSION="${Version}" \
  --build-arg BUILD_TIME="${BuildTime}" \
  --build-arg COMMIT="${GitCommit}" \
  --build-arg MAIN_PATH="${MainPath}" .
}

print_app_info(){
  echo "****************************************"
  echo "App:${Org}:${Name}"
  echo "Version:${Version}"
  echo "Commit:${GitCommit}"
  echo "Build:${BuildTime}"
  echo "Main_Path:${MainPath}"
  echo "****************************************"
  echo ""
}

push_image(){
  echo "****************************************"
  echo "Push:${Org}:${Name}:${Version}"
  echo "****************************************"
  echo ""
  docker push "${Org}/${Name}:${Version}"
}

print_app_info

case  $1 in
    push)
		push_image
        ;;
    *)
		build_image
        ;;
esac
`

// 构建Go服务镜像，go运行环境，编译go可执行文件，打包Docker镜像
const dockerFile = `{{if .HasUI}}FROM node:18.4.0 as ui
ARG NAME
ARG VERSION
WORKDIR /ui_build
ADD ui .
RUN npm install && npm run build

{{end}}FROM golang:1.18 as build
ARG NAME
ARG VERSION
ARG COMMIT
ARG BUILD_TIME
ARG MAIN_PATH
ARG BASE="github.com/Domingor/go-blackbox/version"
ENV GOPROXY=https://goproxy.cn,direct
ARG LDFLAGS=" \
    -X ${BASE}.AppName=${NAME} \
    -X ${BASE}.Version=${VERSION} \
    -X ${BASE}.Commit=${COMMIT} \
    -X ${BASE}.Build=${BUILD_TIME} \
    "
WORKDIR /release
ADD . .{{ if .HasUI }}
COPY --from=ui /ui_build/dist/ static_/
{{ end }}
RUN go mod download && go mod verify
RUN go build -v --ldflags "${LDFLAGS} -X ${BASE}.Compiler=$(go version | sed 's/[ ][ ]*/_/g')" -o ${NAME} ${MAIN_PATH}

FROM Homelander/alpine:3.17 as prod
ARG NAME
EXPOSE 80/tcp
WORKDIR /
COPY --from=build /release/${NAME} /
RUN ln -s /${NAME} /app
CMD [ "/app"]
`

// 基础镜像, 设置了+8时区
// docker build -t {org}/alpine:{version} .
const baseDockerFile = baseDockerfileAlpine

// 基础镜像
const baseDockerfileAlpine = `FROM alpine:3.18
MAINTAINER iConnor
ENV TZ=Asia/Shanghai

RUN apk update \
    && apk add --no-cache tzdata \
    && cp /usr/share/zoneinfo/Asia/Shanghai /etc/localtime \
    && echo "Asia/Shanghai" > /etc/timezone \
    && mkdir /lib64 \
    && ln -s /lib/libc.musl-x86_64.so.1 /lib64/ld-linux-x86-64.so.2
`

const baseDockerfileUbuntu = `FROM ubuntu
MAINTAINER Homelander
ENV TIME_ZONE Asia/Shanghai
 
RUN sed -i s@/archive.ubuntu.com/@/mirrors.aliyun.com/@g /etc/apt/sources.list \
    && apt-get update \
    && apt-get install -y tzdata \
    && ln -snf /usr/share/zoneinfo/$TIME_ZONE /etc/localtime && echo $TIME_ZONE > /etc/timezone \
    && dpkg-reconfigure -f noninteractive tzdata \
    && apt-get clean \
    && rm -rf /tmp/* /var/cache/* /usr/share/doc/* /usr/share/man/* /var/lib/apt/lists/*
`
