##############################
# Build stage
##############################
FROM golang:1.25-alpine AS build

### Copy only src files
WORKDIR /app
COPY ./src/ .

# Prepare env
RUN apk add git

### Build the binary
RUN go build -o ./shopping .

##############################
# Final stage
##############################
FROM alpine

### Copy binary file from build stage
COPY --from=build /app/shopping /usr/bin/shopping
COPY --from=build /app/website ./website

CMD ["shopping"]
