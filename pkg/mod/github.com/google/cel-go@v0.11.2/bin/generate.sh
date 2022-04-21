#!/bin/bash -eu
#
# Copyright 2018 Google LLC
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#      http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# TODO: High-level file comment.

#!/bin/sh

DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"

# Generate AntLR artifacts.
java -Xmx500M -cp ${DIR}/antlr-4.9.1-complete.jar org.antlr.v4.Tool  \
    -Dlanguage=Go \
    -package gen \
    -o ${DIR}/../parser/gen \
    -visitor ${DIR}/../parser/gen/CEL.g4

