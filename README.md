# NeoBase - AI Database Assistant
<img width="1707" alt="Screenshot 2025-02-10 at 7 12 19 PM" src="https://github.com/user-attachments/assets/50c36c8b-52f4-49c8-8b2b-d7b98279aa52" />


**NeoBase** is an AI-powered database assistant that helps you manage, query, and optimize your databases effortlessly. With a sleek Neo Brutalism design and real-time chat functionality, NeoBase makes database visualization intuitive and efficient.

## Screenshots
<img width="1710" alt="Screenshot 2025-02-26 at 6 43 22 PM" src="https://github.com/user-attachments/assets/16522d88-1fa0-449e-ae3b-e1ed489a9ff2" />
<img width="1708" alt="Screenshot 2025-02-26 at 6 50 05 PM" src="https://github.com/user-attachments/assets/573926e4-df31-4ef7-ab8b-4dae6d2d1219" />


## Features

- **AI-Powered Queries**: Generate and optimize SQL queries using natural language prompts.
- **Multi-Database Support**: Connect to PostgreSQL, MySQL, MongoDB, Redis, and more.
- **Real-Time Chat Interface**: Interact with your database like you're chatting with an expert.
- **Neo Brutalism Design**: Bold, modern, and high-contrast UI for a unique user experience.
- **Transaction Management**: Start, commit, and rollback transactions with ease.
- **Query Optimization**: Get AI-driven suggestions to improve query performance.
- **Schema Management**: Create indexes, views, and manage schemas effortlessly.
- **Self-Hosted & Open Source**: Deploy on your infrastructure with full control.

## Supported DBs
- PostgreSQL
- Yugabyte
- MySQL
- ClickHouse

## Planned to be supported DBs
- MongoDB (Priority 1)
- Cassandra (Priority 2)
- Redis (Priority 2)
- Neo4j DB (Priority 3)

## Supported LLM Clients
- OpenAI (Any chat completion model)
- Google Gemini (Any chat completion model)

## Planned to be supported LLM Clients
- Anthropic (Claude 3.5 Sonnet)
- Ollama (Any chat completion model)

## Tech Stack

- **Frontend**: React, Tailwind CSS
- **Backend**: Go (Gin framework)
- **App Used Database**: MongoDB, Redis
- **AI Orchestrator**: OpenAI, Google Gemini
- **Database Drivers**: PostgreSQL, Yugabyte, MySQL, MongoDB, Redis, Neo4j, etc.
- **Styling**: Neo Brutalism design with custom Tailwind utilities


## Getting Started

## How to setup
Read ([SETUP](https://github.com/bhaskarblur/neobase-ai-dba/blob/main/SETUP.md)) to learn how to setup NeoBase on your system.
## Usage

1. **Create a new user in the app**:
   - Open the client app on `http://localhost:5173` in your browser.
   - Admin credentials are set via `ADMIN_USERNAME` and `ADMIN_PASSWORD` environment variables.
   - Creating a new user requires an username, password and user signup secret.
   - User signup secret is generated via Admin credenials by sending a POST request to `api/auth/generate-signup-secret` with admin username & password in the body
   - Use this secret to signup a new user from NeoBase UI.

2. **Add a Database Connection**:
   - Click "Add Connection" in the sidebar.
   - Choose & Enter your database credentials (e.g., host, port, username, password).
   - Click "Save" to add the connection.

3. **Chat with Your Database**:
   - Type natural language prompts in the chat interface (e.g., "Show me all users").
   - View the generated SQL, Other DBs query and results.
   - Paginated results that support large volume of data.

4. **Manage Transactions**:
   - Run the query in transaction mode by clicking "Play" icon button in query.
   - You can also cancel the query by clicking "Cancel" icon button in query.
   - Perform rollbacks by clicking "History" icon button in query.

5. **Optimize Queries**:
   - Get AI-driven suggestions to improve query performance.


## Contributing

We welcome contributions! Here’s how you can help:

1. Fork the repository.
2. Create a new branch (`git checkout -b feature/your-feature`).
3. Commit your changes (`git commit -m 'Add some feature'`).
4. Push to the branch (`git push origin feature/your-feature`).
5. Open a pull request.

See the list of contributors in [CONTRIBUTORS](CONTRIBUTORS) file.

## License

This project is licensed under the MIT License. See the [LICENSE](LICENSE) file for details.


Let me know if you'd like to add or modify anything!
