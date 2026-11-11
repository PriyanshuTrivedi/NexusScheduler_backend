# 🚀 NexusScheduler

### Smart Resource Discovery & Appointment Scheduling Platform

A multi-tenant scheduling platform that connects clients with service providers, simplifies appointment management, and brings AI-powered assistance through **LLMs, MCP, and RAG**.

<p align="center">

**Go** · **gRPC** · **PostgreSQL** · **Redis** · **Gemini** · **Pinecone** · **MCP**

</p>

---

## ✨ Key Features

### 👥 User & Organization Management
- Secure user registration and authentication.
- Dedicated **Resource** and **Client** roles.
- Support for individual service providers and organizations.
- Role-based access control across application operations.

### 🏢 Resource Management & Discovery
- Create and manage resource profiles, services, and offerings.
- Support for online, offline, and hybrid appointments.
- Address-to-coordinate resolution using **LocationIQ**.
- Geographic resource discovery powered by **PostgreSQL/PostGIS**.

### 📅 Availability & Appointment Scheduling
- Configure recurring and one-time availability.
- Discover available appointment slots with timezone support.
- Book, reschedule, and cancel appointments.
- Access upcoming and past booking history.
- Concurrency-safe booking workflows using Redis distributed locks and PostgreSQL constraints.
- Transactional email notifications through **RisuMailer**.

### 🤖 AI-Powered Conversational Assistant
- Natural language interactions powered by **Google Gemini**.
- MCP-based tool calling to interact with application functionality.
- Discover resources, explore availability, and retrieve booking information through conversations.
- Persistent conversation history.

### 📄 Document Management & RAG
- Upload and manage resource-specific PDF documents.
- Store documents using **DBByte object storage**.
- Extract and chunk PDF content for retrieval.
- Generate embeddings using Gemini and store them in **Pinecone**.
- Retrieve relevant document passages through semantic search.
- Inject retrieved context into LLM prompts to answer resource-specific queries.

### 🔌 Model Context Protocol (MCP)
- Expose application capabilities through MCP.
- Tools for resource search, resource details, resource types, availability, and booking-related information.
- Connect the AI assistant with live, structured application data.

### 🔐 Security & Reliability
- JWT-based authentication and role-based authorization.
- API rate limiting.
- Redis-backed distributed locking.
- PostgreSQL constraints for booking consistency.
- Document indexing and retrieval through a dedicated RAG pipeline.

---

## 🧠 AI Architecture: MCP + RAG

NexusScheduler combines two complementary approaches to make its AI assistant useful and context-aware.

| MCP | RAG |
|---|---|
| Interacts with live application data | Retrieves information from uploaded documents |
| Resource discovery and details | Resource-specific knowledge |
| Availability and booking information | Semantic search over PDF content |
| Structured tool execution | Context-aware LLM responses |

**MCP handles what is happening in the application, while RAG provides the knowledge behind each resource.**

---

## 🛠️ Tech Stack

| Layer | Technologies |
|---|---|
| Backend | Go |
| Service Communication | gRPC, Protocol Buffers |
| Database | PostgreSQL, PostGIS |
| Caching & Distributed Locking | Redis |
| Object Storage | DBByte |
| Geocoding | LocationIQ |
| LLM | Google Gemini |
| Embeddings | Gemini Embeddings |
| Vector Database | Pinecone |
| AI Integration | Model Context Protocol (MCP) |
| Authentication | JWT, RBAC |
| Email | RisuMailer |
| Dependency Injection | Uber Fx |
| Testing | GoMock, Testify |

---

## 📂 Repositories

| Component | Repository |
|---|---|
| ⚙️ Backend | [NexusScheduler Backend](https://github.com/PriyanshuTrivedi/NexusScheduler_backend) |
| 🎨 Frontend | [NexusScheduler Frontend](https://github.com/PriyanshuTrivedi/NexusScheduler_frontend) |

---

<p align="center">

### 💡 Built to make resource discovery and appointment scheduling smarter.

**NexusScheduler — Discover. Schedule. Connect.**

</p>
