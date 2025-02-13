package main

import (
	"fmt"
	"math"
)

// Calculator defines an interface for mathematical operations
type Calculator interface {
	Add(a, b float64) float64
	Multiply(a, b float64) float64
}

// MathOps implements Calculator
type MathOps struct{}

// Add performs addition
func (m MathOps) Add(a, b float64) float64 {
	return a + b
}

// Multiply performs multiplication
func (m MathOps) Multiply(a, b float64) float64 {
	return a * b
}

// SquareRoot computes the square root
func SquareRoot(x float64) float64 {
	return math.Sqrt(x)
}

// ExecuteOperations calls multiple functions
func ExecuteOperations(calc Calculator, x, y float64) {
	sum := calc.Add(x, y)
	product := calc.Multiply(x, y)
	root := SquareRoot(sum)

	fmt.Println("Sum:", sum)
	fmt.Println("Product:", product)
	fmt.Println("Square Root of Sum:", root)
}

func main() {
	mathOps := MathOps{}
	ExecuteOperations(mathOps, 4, 9)
}
