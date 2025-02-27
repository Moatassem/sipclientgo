FROM golang:alpine AS build
LABEL maintainer="eng.moatassem@gmail.com"

WORKDIR /sipclient

COPY go.mod go.sum ./
RUN go mod download
RUN go mod verify

# Copy everything except ./audio
COPY . .
RUN rm -rf ./audio
RUN go build -o sipclient .

FROM alpine AS run
LABEL maintainer="eng.moatassem@gmail.com"

RUN mkdir -p /sipclient/audio

COPY --from=build /sipclient/client /sipclient/client
COPY ./audio /sipclient/audio

WORKDIR /sipclient

CMD ["./client"]