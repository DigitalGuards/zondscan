# QRL Zond Explorer Frontend

A Next.js-based frontend for the QRL Zond Explorer, providing a user interface to explore blockchain data.

## Project Structure

```
frontend/
├── app/                    # Next.js 13+ App Router
│   ├── components/         # Shared components
│   │   ├── Alert.tsx      # Alert component for notifications
│   │   ├── AreaChart.tsx  # Chart component for data visualization
│   │   ├── AuthProfileIcon.tsx # User profile icon component
│   │   ├── AuthProvider.tsx    # Authentication provider
│   │   ├── SearchBar.tsx      # Global search component
│   │   └── Sidebar.tsx        # Main navigation sidebar
│   │
│   ├── lib/               # Utility functions
│   │   └── helpers.ts     # Common helper functions (formatting, conversion)
│   │
│   ├── models/           # Data models and types
│   │   └── index.ts      # Shared type definitions
│   │
│   ├── blocks/           # Blocks feature
│   │   ├── layout.tsx    # Blocks layout wrapper
│   │   ├── loading.tsx   # Loading state
│   │   └── [query]/      # Dynamic block routes
│   │       ├── page.tsx  # Block list page
│   │       └── types.ts  # Block-related types
│   │
│   ├── transactions/     # Transactions feature
│   │   ├── layout.tsx    # Layout for transactions
│   │   ├── loading.tsx   # Loading state
│   │   └── [query]/      # Dynamic transaction routes
│   │       ├── page.tsx  # Transaction list page
│   │       ├── TransactionsList.tsx # Transaction list component
│   │       └── types.ts  # Transaction-related types
│   │
│   ├── tx/              # Transaction details feature
│   │   ├── layout.tsx   # Transaction detail layout
│   │   ├── loading.tsx  # Loading state
│   │   └── [query]/     # Dynamic transaction routes
│   │       ├── page.tsx # Transaction detail page
│   │       └── types.ts # Transaction detail types
│   │
│   ├── api/             # API routes
│   │   ├── auth/        # Authentication endpoints
│   │   ├── generate/    # Data generation endpoints
│   │   └── transaction/ # Transaction-related endpoints
│   │       └── [hash]/  # Dynamic transaction API routes
│   │           └── route.ts # Transaction API handler
│   │
│   ├── address/         # Address feature
│   │   └── [query]/     # Dynamic address routes
│   │       ├── page.tsx # Address detail page
│   │       └── types.ts # Address-related types
│   │
│   ├── contracts/       # Smart contracts feature
│   │   ├── layout.tsx   # Contracts layout
│   │   └── page.tsx     # Contracts list page
│   │
│   ├── checker/         # Balance checker tool
│   │   └── page.tsx     # Balance checker page
│   │
│   ├── converter/       # Unit converter tool
│   │   └── page.tsx     # Unit converter page
│   │
│   ├── richlist/        # Rich list feature
│   │   └── page.tsx     # Rich list page
│   │
│   ├── vote/           # Voting feature
│   │   └── page.tsx    # Voting page
│   │
│   ├── globals.css     # Global styles
│   ├── layout.tsx      # Root layout
│   └── page.tsx        # Homepage
│
├── public/             # Static assets
│   ├── ABI.json       # Contract ABIs
│   ├── blockchain-icon.svg
│   ├── contract.svg
│   ├── dark.svg
│   ├── favicon.ico
│   ├── loading.svg
│   ├── lookup.svg
│   ├── partner-handshake-icon.svg
│   ├── receive.svg
│   ├── send.svg
│   ├── stats.svg
│   ├── token.svg
│   └── ...
│
├── config.js          # App configuration
├── middleware.ts      # Next.js middleware
├── next.config.js     # Next.js configuration
├── postcss.config.js  # PostCSS configuration
├── tailwind.config.js # Tailwind CSS configuration
└── tsconfig.json     # TypeScript configuration
```

## Features

### Dashboard
- Overview of blockchain statistics
- Real-time updates for:
  - Wallet count
  - Daily transaction volume
  - Block height
  - Total transactions

### Block Explorer
- View latest blocks
- Block details with transactions
- Pagination support
- Search by block number

### Transaction Explorer
- View latest transactions
- Transaction details including:
  - From/To addresses
  - Value
  - Gas information
  - Timestamp
- Support for different transaction types

### Address Explorer
- Address details and balance
- Transaction history
- Token holdings
- Contract interactions

### Tools
- Balance Checker: Check address balances
- Unit Converter: Convert between different units
- Rich List: View top holders
- Contract Explorer: View and interact with smart contracts

## Key Components

### Sidebar (`components/Sidebar.tsx`)
- Main navigation component
- Fixed position with width of 256px
- Dynamic menu items
- Responsive design

### SearchBar (`components/SearchBar.tsx`)
- Global search functionality
- Support for multiple search types:
  - Address
  - Transaction hash
  - Block number

### TransactionsList (`transactions/[query]/TransactionsList.tsx`)
- Paginated transaction display
- Transaction type indicators
- Value formatting
- Link to transaction details

### Loading States
- Each feature has dedicated loading components
- Skeleton loaders for better UX
- Error boundaries for graceful error handling

## Styling

- Tailwind CSS for styling
- Dark theme with consistent color scheme:
  - Primary background: #1a1a1a
  - Secondary background: #2d2d2d
  - Accent color: #ffa729
  - Text colors: #ffffff, #ffa729, #6c757d
- Responsive design patterns
- Custom animations and transitions

## Development Guidelines

1. **Code Organization**
   - Follow Next.js 13+ App Router conventions
   - Keep components focused and reusable
   - Use TypeScript for type safety
   - Implement proper error boundaries

2. **State Management**
   - Use React hooks for local state
   - Implement loading states
   - Handle errors gracefully
   - Use context where appropriate

3. **API Integration**
   - Use API routes for backend communication
   - Implement proper error handling
   - Cache responses where appropriate
   - Handle loading states

4. **Performance**
   - Optimize images and assets
   - Implement proper caching
   - Use dynamic imports where appropriate
   - Monitor and optimize bundle size

## Getting Started

1. Install dependencies:
```bash
npm install
```

2. Set up environment variables:
```bash
cp .env.example .env.local
```

3. Run development server:
```bash
npm run dev
```

4. Build for production:
```bash
npm run build
```

## Configuration

The app uses several configuration files:

- `config.js`: Environment-specific settings
- `next.config.js`: Next.js configuration
- `tailwind.config.js`: Tailwind CSS configuration
- `postcss.config.js`: PostCSS configuration

## Contributing

1. Follow the existing file structure
2. Maintain TypeScript types
3. Include loading states for async operations
4. Test responsive layouts
5. Update documentation for significant changes
6. Follow the commit message convention:
   - feat: New features
   - fix: Bug fixes
   - docs: Documentation changes
   - chore: Maintenance tasks
   - test: Test-related changes

## Testing

1. Run tests:
```bash
npm test
```

2. Run linting:
```bash
npm run lint
```

## Deployment

1. Build the application:
```bash
npm run build
```

2. Start production server:
```bash
npm start
```

## License

This project is licensed under the terms of the LICENSE file included in the repository.
