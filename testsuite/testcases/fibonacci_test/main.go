package main

import "fmt"

func main() {
    // Generate first 10 Fibonacci numbers
    const n = 10
    fibs := make([]int, n)
    if n > 0 {
        fibs[0] = 0
    }
    if n > 1 {
        fibs[1] = 1
    }
    for i := 2; i < n; i++ {
        fibs[i] = fibs[i-1] + fibs[i-2]
    }
    // Print numbers separated by spaces on a single line
    for i, v := range fibs {
        if i > 0 {
            fmt.Print(" ")
        }
        fmt.Print(v)
    }
    fmt.Println()
}
