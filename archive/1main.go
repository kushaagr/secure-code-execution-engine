package main

import (
	"fmt"
	"secure-exec-engine/sandbox"
)

func main() {
	fmt.Println("🔒 Initializing Secure Code Execution Engine Verification Tests...")

	// Test 1: Standard Operational Flow
	fmt.Println("\n--- Test 1: Standard Python Script ---")
	res1, err := sandbox.RunCode("python", "print('Hello Cisco Engineering!')")
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	} else {
		fmt.Printf("Stdout: %sStderr: %sExitCode: %d\nTimedOut: %t\n", 
			res1.Stdout, res1.Stderr, res1.ExitCode, res1.TimedOut)
	}

	// Test 2: Malicious Infinite Loop Attack Vector
	fmt.Println("\n--- Test 2: Malicious Python Loop (Denial of Service Vector) ---")
	infiniteLoopCode := "import time\nwhile True:\n    pass"
	res2, err := sandbox.RunCode("python", infiniteLoopCode)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	} else {
		fmt.Printf("Stdout: %sStderr: %sExitCode: %d\nTimedOut: %t (SUCCESS: Engine killed execution context)\n", 
			res2.Stdout, res2.Stderr, res2.ExitCode, res2.TimedOut)
	}
}
