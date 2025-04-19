from goserver import GoServer

server = GoServer()

# Enable logging middleware
server.middleware("logging", enabled=True)

# Register a simple hello endpoint
@server.route("/hello", method="GET", description="Get a hello message")
def hello():
    return "Hello, World!"

if __name__ == "__main__":
    print("Starting fastpaze server on port 8080...")
    server.start()