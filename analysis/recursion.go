package analysis

import "github.com/TFMV/surrealcode/types"

type recursionData struct {
	depth     int
	lowlink   int
	inStack   bool
	recursive bool
}

func DetectRecursion(functions map[string]types.FunctionCall) map[string]types.FunctionCall {
	index := 0
	stack := []string{}
	recData := map[string]*recursionData{}

	var tarjan func(caller string, depth int)
	tarjan = func(caller string, depth int) {
		rec := &recursionData{
			depth:   index,
			lowlink: index,
			inStack: true,
		}
		recData[caller] = rec
		index++
		stack = append(stack, caller)

		if fn, exists := functions[caller]; exists {
			for _, callee := range fn.Callees {
				// Check for direct recursion first
				if callee == caller {
					fn.IsRecursive = true
					functions[caller] = fn
					continue
				}

				if _, found := recData[callee]; !found {
					tarjan(callee, depth+1)
					rec.lowlink = min(rec.lowlink, recData[callee].lowlink)
				} else if recData[callee].inStack {
					rec.lowlink = min(rec.lowlink, recData[callee].depth)
				}
			}
		}

		// Root of an SCC
		if rec.lowlink == rec.depth {
			var sccNodes []string
			for {
				n := stack[len(stack)-1]
				stack = stack[:len(stack)-1]
				recData[n].inStack = false
				sccNodes = append(sccNodes, n)
				if n == caller {
					break
				}
			}
			// Mark all nodes in cycle if SCC size > 1
			if len(sccNodes) > 1 {
				for _, n := range sccNodes {
					if fn, exists := functions[n]; exists {
						fn.IsRecursive = true
						functions[n] = fn
					}
				}
			}
		}
	}

	// Process all nodes
	for caller := range functions {
		if _, found := recData[caller]; !found {
			tarjan(caller, 0)
		}
	}

	return functions
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
