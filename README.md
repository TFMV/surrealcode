# SurrealCode: Go Code Analysis Tool

SurrealCode is a static code analysis tool that extracts and visualizes Go code structure, storing the analysis in SurrealDB for advanced querying capabilities.

## Architecture

```ascii
+----------------+ +------------------+ +------------------+
| | | | | |
| Go Source | | Code Parser | | Data Models |
| Files |---->| & Analyzer |---->| & Storage |
| | | | | |
+----------------+ +------------------+ +------------------+
| |
v v
+------------------+ +------------------+
| | | |
| Visualization | | SurrealDB |
| (DOT/D3.js) | | Database |
| | | |
+------------------+ +------------------+
```

## Features

- **Static Analysis**
  - Function call relationships
  - Method detection
  - Recursive function detection (using Tarjan's SCC algorithm)
  - Import dependencies
  - Global variables
  - Struct definitions

- **Visualization**
  - Graphviz DOT format
  - D3.js compatible JSON
  - Color-coded node types
  - Visual indicators for methods and recursive functions

- **Database Integration**
  - SurrealDB storage
  - Graph-based relationships
  - Advanced querying capabilities

## Algorithms

### Recursive Function Detection

Uses Tarjan's Strongly Connected Components (SCC) algorithm for O(V+E) complexity.
