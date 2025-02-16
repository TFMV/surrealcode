package analysis

import "github.com/TFMV/surrealcode/types"

type functionNode struct {
	name    string
	index   int
	lowlink int
	inStack bool
}

func DetectRecursion(functions map[string]types.FunctionCall) map[string]types.FunctionCall {
	index := 0
	stack := []string{}
	recData := map[string]*functionNode{}

	var tarjan func(caller string)
	tarjan = func(caller string) {
		rec := &functionNode{
			name:    caller,
			index:   index,
			lowlink: index,
			inStack: true,
		}
		recData[caller] = rec
		index++
		stack = append(stack, caller)

		if fn, exists := functions[caller]; exists {
			for _, callee := range fn.Callees {
				if callee == caller {
					fn.IsRecursive = true
					functions[caller] = fn
					continue
				}

				if data, found := recData[callee]; !found {
					tarjan(callee)
					rec.lowlink = min(rec.lowlink, recData[callee].lowlink)
				} else if data.inStack {
					rec.lowlink = min(rec.lowlink, data.index)
				}
			}
		}

		if rec.lowlink == rec.index {
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

	for caller := range functions {
		if _, found := recData[caller]; !found {
			tarjan(caller)
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
