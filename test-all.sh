#!/bin/bash

# This is a shell script to run a bunch of regression tests that require
# running sentcli in a fully docker-enabled environment. They'll eventually
# be moved into a golang test package.
wercker=$PWD/wercker
workingDir=$PWD/.werckertests
testsDir=$PWD/tests/projects

# Make sure we have a working directory
mkdir -p "$workingDir"
if [ ! -e "$wercker" ]; then
  go build
fi


basicTest() {
  testName=$1
  shift
  printf "testing %s... " "$testName"
  $wercker --debug $@ --working-dir "$workingDir" > "${workingDir}/${testName}.log"
  if [ $? -ne 0 ]; then
    printf "failed\n"
    cat "${workingDir}/${testName}.log"
    return 1
  else
    printf "passed\n"
  fi
  return 0
}

basicTestFail() {
  testName=$1
  shift
  printf "testing %s... " "$testName"
  $wercker $@ --working-dir "$workingDir" > "${workingDir}/${testName}.log"
  if [ $? -ne 1 ]; then
    printf "failed\n"
    cat "${workingDir}/${testName}.log"
    return 1
  else
    printf "passed\n"
  fi
  return 0
}

testDirectMount() {
  echo -n "testing direct-mount..."
  testDir=$testsDir/direct-mount
  testFile=${testDir}/testfile
  > "$testFile"
  echo "hello" > "$testFile"
  logFile="${workingDir}/direct-mount.log"
  $wercker build "$testDir" --direct-mount --docker-local --working-dir "$workingDir" > "$logFile"
  contents=$(cat "$testFile")
  if [ "$contents" == 'world' ]
      then echo "passed"
      return 0
  else
      echo 'failed'
      cat "$logFile"
      return 1
  fi
}

testScratchPush () {
  echo -n "testing scratch-n-push.."
  testDir=$testsDir/scratch-n-push
  logFile="${workingDir}/scratch-n-push.log"
  grepString="uniqueTagFromTest"
  docker images | grep $grepString | awk '{print $3}' | xargs -n1 docker rmi -f > /dev/null 2>&1
  $wercker build "$testDir" --docker-local --working-dir "$workingDir" > "$logFile" && docker images | grep -q "$grepString"
  if [ $? -eq 0 ]; then
    echo "passed"
    return 0
  else
      echo 'failed'
      cat "$logFile"
      docker images
      return 1
  fi
}


runTests() {
  basicTest "source-path" build "$testsDir/source-path" || return 1
  basicTest "local services" build "$testsDir/local-service/service-consumer" || return 1
  basicTest "deploy" deploy "$testsDir/deploy-no-targets" --docker-local || return 1
  basicTest "deploy target" deploy --deploy-target test "$testsDir/deploy-targets" --docker-local || return 1
  basicTest "after steps" build --pipeline build_true "$testsDir/after-steps-fail" --docker-local || return 1
  basicTest "relative symlinks" build "$testsDir/relative-symlinks" --docker-local || return 1

  # this one will fail but we'll grep the log for After-step passed: test
  basicTestFail "after steps fail" --no-colors build --pipeline build_fail "$testsDir/after-steps-fail" --docker-local || return 1
  grep -q "After-step passed: test" "${workingDir}/after steps fail.log" || return 1

  # make sure we get some human understandable output if the wercker file is wrong
  basicTestFail "empty wercker file" build "$testsDir/invalid-config" --docker-local|| return 1
  grep -q "Your wercker.yml is empty." "${workingDir}/empty wercker file.log" || return 1

  basicTest "multiple services with the same image" build || return 1

  testDirectMount || return 1
  testScratchPush || return 1

  # test runs locally but not in wercker build container
  basicTest "shellstep" build --enable-dev-steps "$testsDir/shellstep" || return 1

  # test deploy behavior with different levels of specificity
  cd "$testsDir/local-deploy/latest-no-yml"
  basicTest "local deploy using latest build not containing wercker.yml" deploy --docker-local || return 1
  cd "$testsDir/local-deploy/latest-yml"
  basicTest "local deploy using latest build containing wercker.yml" deploy --docker-local || return 1
  cd "$testsDir/local-deploy/specific-no-yml"
  basicTest "local deploy using specific build not containing wercker.yml" deploy --docker-local ./last_build || return 1
  cd "$testsDir/local-deploy/specific-yml"
  basicTest "local deploy using specific build containing wercker.yml" deploy --docker-local ./last_build || return 1
}

runTests
rm -rf "$workingDir"
