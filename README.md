# surrealcode

surrealcode is a tool that analyzes Go code and stores the results in a SurrealDB database as a graph.

The plan is to use the graph to visualize the code and the relationships between the functions, structs, and variables, measure the complexity of the code, and identify the potential areas for improvement.

Specifically, my goal is to use this in a CICD pipeline to monitor code quality and complexity over time and provide relational context to LLMs for code generation and maintenance tasks.

## Status

Early stage, just a proof of concept.

## Usage

```bash
go run main.go
```

## License

MIT
