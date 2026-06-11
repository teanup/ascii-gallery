FROM golang:1.21 AS BUILD_IMAGE

ENV GO111MODULE=on

WORKDIR /app

COPY go.mod .
COPY go.sum .
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -a -ldflags '-extldflags "-static"' -o ascii

FROM scratch

COPY --from=BUILD_IMAGE /app/ascii /ascii
COPY --from=BUILD_IMAGE /app/ansi_frames/ /ansi_frames/

EXPOSE $PORT

ENTRYPOINT ["/ascii"]
