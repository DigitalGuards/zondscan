# QRL Zond Explorer Frontend

A Next.js-based frontend for the QRL Zond Explorer, providing a user interface to explore blockchain data.

## Project Structure

```
frontend/
├── app/                    # Next.js 13+ App Router
│   ├── components/         # Shared components
│   │   ├── Alert.tsx
│   │   ├── AreaChart.tsx
│   │   ├── AuthProfileIcon.tsx
│   │   └── Sidebar.tsx    # Main navigation sidebar
│   │
│   ├── blocks/            # Blocks feature
│   │   ├── layout.tsx     # Blocks layout wrapper
│   │   ├── loading.tsx    # Loading state
│   │   └── [query]/       # Dynamic block routes
│   │       ├── page.tsx   # Block list page
│   │       └── types.ts   # Block-related types
│   │
│   ├── transactions/      # Transactions feature
│   │   ├── layout.tsx
│   │   ├── loading.tsx
│   │   └── [query]/
│   │       ├── page.tsx
│   │       ├── TransactionsList.tsx
│   │       └── types.ts
│   │
│   ├── contracts/         # Smart contracts feature
│   │   ├── layout.tsx
│   │   └── page.tsx
│   │
│   ├── checker/          # Balance checker tool
│   │   └── page.tsx
│   │
│   ├── converter/        # Unit converter tool
│   │   └── page.tsx
│   │
│   ├── richlist/         # Rich list feature
│   │   └── page.tsx
│   │
│   ├── layout.tsx        # Root layout (includes Sidebar)
│   └── page.tsx          # Homepage
│
├── public/               # Static assets
│   ├── ABI.json         # Contract ABIs
│   ├── blockchain-icon.svg
│   └── ...
│
├── lib/                 # Utility functions
│   └── helpers.ts       # Common helper functions
│
├── types/               # Global TypeScript types
└── config.js           # App configuration
```

## Layout System

The app uses a nested layout structure:

1. **Root Layout** (`app/layout.tsx`)
   - Provides the base structure
   - Includes the Sidebar component
   - Handles the main grid layout with sidebar spacing

2. **Feature Layouts** (e.g., `blocks/layout.tsx`, `transactions/layout.tsx`)
   - Wrap specific feature content
   - Handle feature-specific spacing and styling

## Key Components

### Sidebar (`components/Sidebar.tsx`)
- Main navigation component
- Fixed position with width of 256px (w-64)
- Contains links to all major features

### Block Components
- `BlockCard`: Displays individual block information
- Uses a flex layout with responsive design
- Maximum width constraints to prevent content stretching

### Transaction Components
- `TransactionsList`: Manages transaction display and pagination
- `TransactionCard`: Individual transaction display

## Styling

- Uses Tailwind CSS for styling
- Dark theme with consistent color scheme:
  - Background: #1a1a1a
  - Accent: #ffa729
  - Card backgrounds: #2d2d2d
  - Hover states: #3d3d3d

## Page Organization

1. **List Pages** (e.g., blocks/[query]/page.tsx)
   - Display paginated lists of items
   - Include navigation controls
   - Maximum width constraints for readability

2. **Detail Pages** (e.g., block/[id]/page.tsx)
   - Show detailed information for individual items
   - Responsive layouts for different screen sizes

3. **Tool Pages** (e.g., checker/, converter/)
   - Utility tools for specific functions
   - Self-contained functionality

## Development Guidelines

1. **Layout Consistency**
   - Use the root layout's sidebar spacing (ml-64)
   - Avoid duplicate margin/padding in nested layouts
   - Maintain consistent max-width constraints

2. **Component Structure**
   - Keep components focused and single-responsibility
   - Use TypeScript interfaces for props
   - Implement loading states

3. **Styling Best Practices**
   - Use Tailwind utility classes
   - Maintain dark theme consistency
   - Follow responsive design patterns

4. **State Management**
   - Use React hooks for local state
   - Implement proper loading and error states
   - Handle pagination efficiently

## Getting Started

1. Install dependencies:
```bash
npm install
```

2. Run development server:
```bash
npm run dev
```

3. Build for production:
```bash
npm run build
```

## Configuration

The app uses `config.js` for environment-specific settings:
- API endpoints
- Feature flags
- Environment variables

## Contributing

1. Follow the existing file structure
2. Maintain TypeScript types
3. Include loading states for async operations
4. Test responsive layouts
5. Update documentation for significant changes
