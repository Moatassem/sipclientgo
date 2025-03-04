FROM golang:alpine AS build
LABEL maintainer="eng.moatassem@gmail.com"

WORKDIR /sipclient

COPY go.mod go.sum ./
RUN go mod download
RUN go mod verify

# Copy everything except ./audio
COPY . .
RUN rm -rf ./audio
RUN rm -rf ./webserver/portal
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


# docker build -t sipclient .
# docker run -d --name sipclient -e http_port="8085" -p 8085:8085 -p 5067:5067/udp -p 5069:5069/udp sipclient:latest