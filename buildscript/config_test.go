package buildscript

import "testing"

func TestBuild(t *testing.T) {

	//GenerateBaseDockerfile()

	Generate("test", "cloudbyte.top", "com/main.go", true)
}
