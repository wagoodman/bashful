# -*- coding: utf-8 -*-
import subprocess
import threading
import collections
from reprint import output, ansi_len
import sys
import select
import shlex
import time
import yaml
import sys
from functools import partial

MAX_NAME_LEN = 0
INDENT = 0

Result = collections.namedtuple("Result", "name cmd returncode stderr")

class Color:
    PURPLE = '\033[95m'
    BLUE = '\033[94m'
    GREEN = '\033[92m'
    YELLOW = '\033[93m'
    RED = '\033[91m'
    NORMAL = '\033[0m'
    BOLD = '\033[1m'
    UNDERLINE = '\033[4m'

#TEMPLATE = "{title:{width}s} ❭ {color}{msg}{reset}"
TEMPLATE = "{title:{width}s}    {color}{msg}{reset}"
PARALLEL_TEMPLATE = "├── " + TEMPLATE
LAST_PARALLEL_TEMPLATE = "└── " + TEMPLATE

def exec_task(output_lines, idx, name, cmd, results, indent=False, last=False):
    if indent and last:
        template = LAST_PARALLEL_TEMPLATE
        offset = -4
    elif indent:
        template = PARALLEL_TEMPLATE
        offset = -4
    else:
        template = TEMPLATE
        offset = 0

    width = MAX_NAME_LEN+INDENT+offset
    width += len(name)-ansi_len(name)
    p = subprocess.Popen(shlex.split(cmd), stdout=subprocess.PIPE, stderr=subprocess.PIPE)
    output_lines[idx] = template.format(title=name, width=width, msg='Starting...', color=Color.YELLOW, reset=Color.NORMAL)
    error = []
    while True:
        reads = [p.stdout.fileno(), p.stderr.fileno()]
        ret = select.select(reads, [], [])

        for fd in ret[0]:
            if fd == p.stdout.fileno():
                read = p.stdout.readline()
                output_lines[idx] = template.format(title=name, width=width, msg=read, color=Color.NORMAL, reset=Color.NORMAL)

            elif fd == p.stderr.fileno():
                read = p.stderr.readline()
                error.append(read.rstrip())
                output_lines[idx] = template.format(title=name, width=width, msg=" ".join(read.split('\n')), color=Color.RED, reset=Color.NORMAL)

        if p.poll() != None:
            break


    read = p.stdout.readline()
    output_lines[idx] = template.format(title=name, width=width, msg=read, color=Color.NORMAL, reset=Color.NORMAL)

    read = p.stderr.readline()
    error.append(read.rstrip())
    output_lines[idx] = template.format(title=name, width=width, msg=" ".join(read.split('\n')), color=Color.RED, reset=Color.NORMAL)


    if p.returncode != 0:
        if len(error) > 0:
            output_lines[idx] = template.format(title=name, width=width, msg="✘ Error (%d): stderr to follow..." % p.returncode, color=Color.RED, reset=Color.NORMAL)
        else:
            output_lines[idx] = template.format(title=name, width=width, msg="✘ Error (%d)" % p.returncode, color=Color.RED, reset=Color.NORMAL)
    else:
        output_lines[idx] = template.format(title=name, width=width, msg="✔ Complete", color=Color.GREEN, reset=Color.NORMAL)

    results[idx] = Result(name, cmd, p.returncode, "\n".join(error))


def run_tasks(tasks, title=None):
    length, offset = len(tasks), 0
    if title:
        offset = 1
    with output(output_type='list', initial_len=length+offset, interval=0) as output_lines:
        if title:
            output_lines[0] = "%s%s%s"%(Color.BOLD, title,Color.NORMAL)
        proc = []
        results = [None]*(length+offset)
        MAX_NAME_LEN = max([len(name) for name, cmd in tasks.items()])
        for idx, (name, cmd) in enumerate(tasks.items()):
            time.sleep(0.01)

            p = threading.Thread(target=exec_task, args=(output_lines, idx+offset, name, cmd, results, len(tasks)!=1, idx==len(tasks)-1))
            proc.append(p)
            p.start()
        [p.join() for p in proc]

    for result in results:
        if result != None and result.returncode != 0:
            print "\n%s%sTask %s finished with error: %s%s" % (Color.BOLD,Color.RED, repr(result.name), result.returncode, Color.NORMAL)
            if result.stderr:
                print "%s%s%s" % (Color.RED, result.stderr.strip(), Color.NORMAL)

def process_task(options, bold_name=False):
    if isinstance(options, dict):
        if 'name' in options and 'cmd' in options:
            global MAX_NAME_LEN
            name = options['name']
            if bold_name:
                name = "%s%s%s" % (Color.BOLD, options['name'], Color.NORMAL)

            MAX_NAME_LEN = max(MAX_NAME_LEN, ansi_len(name) )
            return name, options['cmd']
        elif 'cmd' in options:
            return options['cmd'], options['cmd']
        else:
            raise RuntimeError("Task requires a name and cmd")
    return options, options


def build_serial(options):
    name, cmd = process_task(options, bold_name=True)
    return partial(run_tasks, {name: cmd})

def build_parallel(options):
    global INDENT
    INDENT = 3
    tasks = collections.OrderedDict()
    title = None
    if 'title' in options:
        title = options['title']
    if 'tasks' not in options:
        raise RuntimeError('Parallel option requires tasks')
    for task_options in options['tasks']:
        name, cmd = process_task(task_options)
        tasks[name] = cmd
    return partial(run_tasks, tasks, title=title)

def builder(obj):
    ret = []
    for item in obj:
        for definition, options in item.items():
            if definition == 'task':
                ret.append(build_serial(options))
            if definition == 'parallel':
                ret.append(build_parallel(options))
    return ret

def main():
    obj = yaml.load(open(sys.argv[1],'r').read())
    task_funcs = builder(obj)
    for func in task_funcs:
        func()


main()
