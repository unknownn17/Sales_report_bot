# Telegram Sales & Inventory Bot (Go + MongoDB)

Telegram bot for managing sales, stock per size, debts, installment payments, and profit reports for clothing resale businesses. Built end-to-end in Go (1.22+) using `gopkg.in/telebot.v3` and the official MongoDB Go driver.

---

## 🌟 Key Features

1. **Intelligent Uzbek Post Parser (Flow 1: ➕ Mahsulot qo'shish)**:
   - Forwards or photos with captions in Uzbek are automatically parsed using regex.
   - Extracts `SellPrice` (formats: `240 000 so'm`, `240,000`, `240.000`), `Sizes` (XS...XXXXL, 42, 44, 46...), `Brand`, and `Name`.
   - Fetches live USD→UZS exchange rates from the Central Bank of Uzbekistan (CBU) API with caching and manual override support.
   - Per-size stock tracking with atomic persistence.
   - Interactive summary with inline field edit buttons before saving.

2. **Atomic Stock Reservation & Multi-type Sales (Flow 2: 💰 Sotuvni tasdiqlash)**:
   - Searchable and paginated active product browser.
   - Live stock preview per size with out-of-stock indicators.
   - **Atomic stock decrement** with `$inc` and `$elemMatch` preventing concurrency overselling.
   - Automatic **stock rollback** on `/cancel`.
   - Payment types:
     - 💵 **Naqd (Cash)**: Fully paid.
     - 💳 **Karta (Card)**: Fully paid.
     - 📝 **Nasiya (Credit)**: Buyer name, contact, initial payment, and due date tracking.

3. **Debtor Tracking & Installment Payments (Flow 3: 📋 Qarzdorlar / 🔔 Muddati yaqin)**:
   - Live debtor list showing remaining balance and due dates.
   - 1-tap payment recording with auto balance updates and payment receipts.
   - "Muddati yaqin" filter for debts due within the next N days.

4. **Accurate Profit Calculation & Aggregation Reports (📊 Bugungi / 📈 Davriy hisobot)**:
   - Aggregation pipeline over MongoDB `sales` collection.
   - Splits profit proportionally between **Olingan foyda (Received)** and **Kutilayotgan foyda (Pending)** for credit sales:
     $$\text{Received Profit} = \sum (\text{SellPrice} - \text{CostPriceSom}) \times \frac{\text{AmountPaid}}{\text{SellPrice} \times \text{Quantity}} \times \text{Quantity}$$
   - Displays top selling products for the period.

---

## 📂 Project Structure

```
├── bot/
│   ├── handlers/
│   │   ├── common.go          # AppContext, cancel logic & text input routing
│   │   ├── add_product.go     # Flow 1: Add product post parser & save
│   │   ├── confirm_sale.go    # Flow 2: Sale flow & stock decrement
│   │   ├── payments.go        # Flow 3: Installment payment collection
│   │   ├── reports.go         # Profit reports & top sellers
│   │   ├── debtors.go         # Debtors and due soon list
│   │   └── stock.go           # Inventory overview & low stock alerts
│   ├── state/
│   │   └── session.go         # Thread-safe per-admin FSM session manager
│   ├── bot.go                 # Telebot setup & admin auth middleware
│   └── menu.go                # Admin reply keyboard menu layout
├── config/                    # (optional / env loader)
├── db/
│   └── mongo.go               # Mongo connection, ping, collection index setup
├── models/
│   ├── product.go             # Product and StockItem models
│   ├── sale.go                # Sale model
│   └── payment.go             # Payment model
├── repository/
│   ├── products.go            # Products CRUD, atomic decrement & increment
│   ├── sales.go               # Sales aggregation pipelines & queries
│   └── payments.go            # Payments collection queries
├── services/
│   ├── parser.go              # Uzbek text post regex parser
│   ├── parser_test.go         # Parser unit tests
│   ├── exchangerate.go        # CBU USD exchange rate fetcher & cache
│   └── profit.go              # Date ranges & Uzbek formatting utilities
├── docker-compose.yml         # Local MongoDB service
├── go.mod                     # Go module dependencies
├── main.go                    # Application entry point
├── .env.example               # Environment variables template
└── README.md
```

---

## 🚀 Getting Started

### 1. Requirements
- **Go** 1.22 or newer
- **Docker & Docker Compose** (or a running MongoDB instance)

### 2. Configure Environment Variables
Copy `.env.example` to `.env`:
```bash
cp .env.example .env
```
Fill in your actual values:
- `BOT_TOKEN`: Telegram bot token from [@BotFather](https://t.me/BotFather).
- `ADMIN_IDS`: Comma-separated Telegram user IDs (obtain via [@userinfobot](https://t.me/userinfobot)).
- `MONGO_URI`: `mongodb://localhost:27017`

### 3. Start MongoDB
Run MongoDB in the background with Docker Compose:
```bash
docker-compose up -d
```

### 4. Run Unit Tests
```bash
go test -v ./...
```

### 5. Start the Bot
```bash
go run main.go
```

---

## 🔒 Access Control
Only user IDs listed in `ADMIN_IDS` can interact with the bot. Unauthorized users receive a polite Uzbek denial message and are blocked by middleware from accessing any handler.
