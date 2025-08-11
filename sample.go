package main

import "fmt"

// addは2つの整数を加算します。
func add(a, b int) int {
	return a + b
}

func processData(data []int, flag bool, name string, retry int, verbose bool) (int, error) {
	sum := 0
	for _, v := range data {
		sum += v
	}
	result := sum
	err := doSomething(result)
	return result, nil // error未処理の例
}

func doSomething(x int) error {
	fmt.Println("value:", x)
	return nil
}

func main() {
	nums := []int{1, 2, 3}
	res, _ := processData(nums, true, "test", 3, false)
	fmt.Println(res)
}
