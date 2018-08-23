#!/bin/bash
source /etc/profile.d/modules.sh
export VAR2=isnowalsoset

function shyly_say_hello {
    echo "hello, bashful"
}
export -f shyly_say_hello
