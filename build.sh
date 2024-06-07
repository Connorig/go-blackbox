#!/bin/bash
Name="test24.6.7"
MainPath="com/main.go"
Org="cloudbyte.top"

# shellcheck disable=SC2046
Version=$(git describe --tags $(git rev-list --tags --max-count=1))
# shellcheck disable=SC2154
GitCommit=$(git log --pretty=format:"%h" -1)
BuildTime=$(date +%FT%T%z)

build_image(){
  git checkout "${Version}"
  docker build -t "${Org}/${Name}:${Version}" \
  --build-arg NAME="${Name}" \
  --build-arg VERSION="${Version}" \
  --build-arg BUILD_TIME="${BuildTime}" \
  --build-arg COMMIT="${GitCommit}" \
  --build-arg MAIN_PATH="${MainPath}" .
}

print_app_info(){
  echo "****************************************"
  echo "App:${Org}:${Name}"
  echo "Version:${Version}"
  echo "Commit:${GitCommit}"
  echo "Build:${BuildTime}"
  echo "Main_Path:${MainPath}"
  echo "****************************************"
  echo ""
}

push_image(){
  echo "****************************************"
  echo "Push:${Org}:${Name}:${Version}"
  echo "****************************************"
  echo ""
  docker push "${Org}/${Name}:${Version}"
}

print_app_info

case  $1 in
    push)
		push_image
        ;;
    *)
		build_image
        ;;
esac
