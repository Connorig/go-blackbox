package buildscript

import "testing"

func TestBuild(t *testing.T) {

	//GenerateBaseDockerfile()

	Generate("test24.6.7", "cloudbyte.top", "com/main.go", false)
}
