#!/usr/bin/env bash
set -u
#if="$1"             # the input file
#lct=$(wc -l <"$if") # number of lines in input file
lct=$1
tot=${lct:-0}       # total number of itterations; If unknown, default is 0
                    #+  The total is know in this case. '$tot' is just a rough
                    #+  example of how to suppress the progress %age output
beg=$(date +%s.%N)  # starting unix time.%N is nanoseconds (a GNU extension)
swx=10              # keep a sliding window of max 'n' itteratons (to average)
unset sw            # an array of the last '$swx' rates
for i in $(seq 1 $lct) ;do
    sw[$i]=$(date +%s.%N)  # sliding window start time
    # ================
      sleep .$RANDOM       #  ... process something here
    # ================
    now=$(date +%s.%N)     # current unix time
    if ((i<=swx)) ;then
        sw1=1              # first element of sliding window
        sww=$i             # sliding window width (starts from 1)
    else
        sw1=$((i-swx+1))
        sww=$swx
    fi
    # bc=($(bc <<<"scale=2; $i/($now-$beg); $sww/($now-${sw[$sw1]})"))
    # oavg=${bc[0]}                  # overall average rate
    # swhz=${bc[1]}                  # sliding window rate
    # ((i>swx)) && unset sw[$sw1-1]  # remove old entry from sliding window list
    # ((tot==0)) && pc= || pc="progress: $(bc <<<"scale=1; x=($i*100)/$tot; if (x<1) print 0; x")%"
    # msg="window: $swhz/s   overall: $oavg/s   $pc"
    #printf "\r%"$((${#i}+1))"s=\r%s" "" "$msg"
    # echo "$msg (from $1)"
    #echo "The start..." .$RANDOM .$RANDOM .$RANDOM .$RANDOM .$RANDOM .$RANDOM .$RANDOM .$RANDOM .$RANDOM .$RANDOM .$RANDOM .$RANDOM .$RANDOM .$RANDOM .$RANDOM .$RANDOM .$RANDOM .$RANDOM .$RANDOM .$RANDOM .$RANDOM .$RANDOM .$RANDOM .$RANDOM .$RANDOM .$RANDOM .$RANDOM .$RANDOM .$RANDOM .$RANDOM .$RANDOM .$RANDOM .$RANDOM .$RANDOM .$RANDOM .$RANDOM .$RANDOM .$RANDOM .$RANDOM .$RANDOM .$RANDOM .$RANDOM .$RANDOM .$RANDOM .$RANDOM .$RANDOM .$RANDOM .$RANDOM .$RANDOM .$RANDOM .$RANDOM .$RANDOM .$RANDOM .$RANDOM .$RANDOM .$RANDOM .$RANDOM .$RANDOM .$RANDOM .$RANDOM .$RANDOM .$RANDOM .$RANDOM .$RANDOM .$RANDOM .$RANDOM .$RANDOM .$RANDOM .$RANDOM .$RANDOM .$RANDOM .$RANDOM .$RANDOM .$RANDOM .$RANDOM .$RANDOM .$RANDOM .$RANDOM .$RANDOM .$RANDOM .$RANDOM .$RANDOM .$RANDOM .$RANDOM .$RANDOM .$RANDOM .$RANDOM .$RANDOM .$RANDOM .$RANDOM .$RANDOM .$RANDOM
    echo "The start at $1..." .$RANDOM .$RANDOM .$RANDOM .$RANDOM .$RANDOM .$RANDOM .$RANDOM .$RANDOM .$RANDOM .$RANDOM .$RANDOM .$RANDOM 
    #echo "Something $i Another (from $1)"
    #pkill -15 -f jitter
    #echo $?
done
