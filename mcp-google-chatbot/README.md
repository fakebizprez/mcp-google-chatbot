# MCP Google Chatbot

Freight carrier risk assessment bot for Google Chat.

## Prerequisites

- Go 1.21+
- Docker
- Google Cloud Account and a configured Service Account

## Setup

1.  **Clone the repository:**
    ```bash
    git clone <repository-url>
    cd mcp-google-chatbot
    ```

2.  **Set up environment variables:**
    Copy `.env.example` to `.env` and fill in the required values:
    ```bash
    cp .env.example .env
    ```
    See `.env.example` for required variables.

3.  **Google Service Account:**
    Ensure the `GOOGLE_APPLICATION_CREDENTIALS` environment variable points to the path of your service account JSON key file.

## Running Locally

### Using Docker
```bash
docker-compose up --build
```
The application will be accessible at `http://localhost:8080`.

### Running directly with Go
```bash
go run main.go
```

## Google Chat App Configuration

- **App Name**: MCP Google Chatbot
- **Avatar URL**: `[PLACEHOLDER_AVATAR_URL]`
- **Description**: Freight carrier risk assessment bot for Google Chat.
- **Functionality**:
    - Can send messages.
    - Can be added to spaces.
    - Can respond to @mentions.
- **Connection Settings**:
    - **App Type**: Bot
    - **Configuration Type**: HTTP URL
    - **URL**: `https://[PLACEHOLDER_DOMAIN]/webhook` (replace `[PLACEHOLDER_DOMAIN]` with your deployment domain)
    - **Authentication**: Service account (ensure the bot's service account has `chat.bot` scope and is added as a Chat app).
- **Permissions**:
    - `chat.bot`
- **Slash Commands**:
    - Command: `/carrier`
    - Description: Fetches carrier risk assessment.
    - Parameters: `[MC_NUMBER]` (e.g., `/carrier 123456`)
- **Interactive Features**:
    - Enable cards and buttons.

## Placeholders
Remember to replace the following placeholders in your configuration and documentation:
- `[PLACEHOLDER_GOOGLE_PROJECT_ID]`
- `[PLACEHOLDER_SERVICE_ACCOUNT_EMAIL]`
- `[PLACEHOLDER_DOMAIN]`
- `[PLACEHOLDER_AVATAR_URL]`
- `[PLACEHOLDER_MCP_API_BEARER_TOKEN]`
- `[PLACEHOLDER_MCP_API_REFRESH_TOKEN]`
- `[PLACEHOLDER_MCP_API_TOKEN_ENDPOINT]`
