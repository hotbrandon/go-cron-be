# Docker compose and environmetn variables
You need to explicitly define `env_file` in your docker-compose.yaml if you want to use .env files for individual services.
```yaml
env_file:
    - .env 
```    

Docker Compose does NOT automatically pick up .env files for individual services. Here's how it actually works:
What Docker Compose Does Automatically
Docker Compose will automatically use .env file for variable substitution within the compose file itself:
# docker-compose.yaml
```yaml
services:
  go-cron:
    image: myapp:${VERSION}  # This will use VERSION from .env
    ports:
      - "${PORT}:8080"       # This will use PORT from .env
```

# What You Need to Do for Service Environment Variables
For the service container to receive environment variables from .env, you must explicitly specify:
```yaml
services:
  go-cron:
    build: .
    env_file:
      - .env  # ← This is REQUIRED
```
