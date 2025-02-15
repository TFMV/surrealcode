# SurrealCode: Go Code Analysis Tool

SurrealCode is a static code analysis tool designed to extract, analyze, and visualize Go code structure. It integrates with **SurrealDB** for storage and querying of code relationships.

## 📐 Architecture

```ascii
+----------------+      +------------------+      +------------------+
|                |      |                  |      |                  |
|  Go Source     | ───> |  Code Parser     | ───> |  Data Models &   |
|  Files         |      |  & Analyzer      |      |  Storage         |
|                |      |                  |      |                  |
+----------------+      +------------------+      +------------------+
       |                         |
       v                         v
+------------------+      +------------------+
|                  |      |                  |
|  Visualization   |      |  SurrealDB        |
|  (DOT/D3.js)    |      |  Graph Database   |
|                  |      |                  |
+------------------+      +------------------+
```
