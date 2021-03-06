# The base go-image
FROM golang:1.14-alpine
 
# Create a directory for the app
RUN mkdir /app
 
# Copy all files from the current directory to the app directory
COPY . /app
 
# Set working directory
WORKDIR /app
 
# Run command as described:
RUN	go build -o bin/app cmd/main.go
 
# Run the server executable
CMD [ "/app/bin/app" ]
