#!/usr/bin/env bash

count=$1
for i in $(seq 1 $count) ;do
    sleep .$RANDOM
    echo "$2 worker-with-error ($i/$1): $(cat /dev/urandom | tr -dc 'a-zA-Z0-9' | fold -w $((RANDOM%80 + 1)) | head -n 1)"
done

>&2 echo "This is a specific error error"
>&2 echo '  File "runner.py", line 49'
>&2 echo '    output_lines[idx] = TEMPLATE.format(title=name, width=titlelen, msg="stderr: " + " "..join(read.split('\n')), color=Color.RED, reset=Color.NORMAL)'
>&2 echo 'SyntaxError: invalid syntax'
>&2 echo "Could not complete due to an error"
exit 1
