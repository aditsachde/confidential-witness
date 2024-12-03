# https://github.com/GoogleContainerTools/distroless/blob/main/examples/go/Dockerfile
FROM golang:1.23 AS build

WORKDIR /go/src/app

COPY go.mod go.sum .
RUN go mod download

COPY ./cmd/confidential-witness .
RUN CGO_ENABLED=0 go build -o /go/bin/app

FROM gcr.io/distroless/static-debian12

COPY --from=build /go/bin/app /

# Exposed ports must be defined in the Dockerfile
# https://cloud.google.com/confidential-computing/confidential-space/docs/create-customize-workloads#inbound-ports
# Witness port
EXPOSE 80/tcp
# Public key served on this port
EXPOSE 8080/tcp

# The settable environment variables must be explicity declared here
# https://cloud.google.com/confidential-computing/confidential-space/docs/create-customize-workloads#launch_policies
LABEL "tee.launch_policy.allow_env_override"="WITNESS_KEY,WITNESS_NAME,WITNESS_AUDIENCE"

CMD ["/app"]
