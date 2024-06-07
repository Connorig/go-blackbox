FROM golang:1.18 as build
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
ADD buildscript .
RUN go mod download && go mod verify
RUN go build -v --ldflags "${LDFLAGS} -X ${BASE}.Compiler=$(go version | sed 's/[ ][ ]*/_/g')" -o ${NAME} ${MAIN_PATH}

FROM Homelander/alpine:3.17 as prod
ARG NAME
EXPOSE 80/tcp
WORKDIR /
COPY --from=build /release/${NAME} /
RUN ln -s /${NAME} /app
CMD [ "/app"]
