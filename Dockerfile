FROM golang:alpine AS build
LABEL maintainer="eng.moatassem@gmail.com"

WORKDIR /sipclient

COPY go.mod go.sum ./
RUN go mod download
RUN go mod verify

COPY . .
RUN rm -rf ./audio
RUN rm -rf ./webserver/portal
RUN rm data.json
RUN go build -o sipclient .

FROM alpine AS run
LABEL maintainer="eng.moatassem@gmail.com"

RUN mkdir -p /sipclient/audio

COPY --from=build /sipclient/sipclient /sipclient/sipclient
COPY ./audio /sipclient/audio
COPY ./data.json /sipclient/data.json
COPY ./webserver/portal /sipclient/webserver/portal

WORKDIR /sipclient

CMD ["./sipclient"]