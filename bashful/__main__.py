#!/usr/bin/env python
# -*- coding: utf-8 -*-
"""bashful
Because your bash scripts should be quiet and shy (and not such a loudmouth).

Usage:
  bashful <ymlfile>
  bashful (-h | --help)
  bashful --version

Options:
  -h  --help         Show this screen.
  -v  --version      Show version.
"""
from docopt import docopt
from functools import partial
import collections
import subprocess
import threading
import select
import shlex
import time
import yaml
import sys
import io

from bashful.version import __version__
from bashful.reprint import output, ansi_len, preprocess, no_ansi

CAP_NAME_LEN = 30
MAX_NAME_LEN = 0
INDENT = 0
INDENT_LEN = 8

TEMPLATE               = " {color}{status}{reset} {title} {msg}"
PARALLEL_TEMPLATE      = " {color}{status}{reset}  ├─ {title} {msg}"
LAST_PARALLEL_TEMPLATE = " {color}{status}{reset}  └─ {title} {msg}"


Result = collections.namedtuple("Result", "name cmd returncode stderr")

EXIT = False

class Color:
    PURPLE = '\033[95m'
    BLUE = '\033[94m'
    GREEN = '\033[92m'
    YELLOW = '\033[93m'
    RED = '\033[91m'
    NORMAL = '\033[0m'
    BOLD = '\033[1m'
    UNDERLINE = '\033[4m'
    INVERSE = '\033[7m'

def exec_task(output_lines, idx, name, cmd, results, indent=False, last=False):
    global EXIT
    if indent and last:
        template = LAST_PARALLEL_TEMPLATE
        offset = -INDENT_LEN
    elif indent:
        template = PARALLEL_TEMPLATE
        offset = -INDENT_LEN
    else:
        template = TEMPLATE
        offset = 0

    # width = MAX_NAME_LEN+INDENT+offset
    # width += len(name)-ansi_len(name)

    p = subprocess.Popen(shlex.split(cmd), stdout=subprocess.PIPE, stderr=subprocess.PIPE)

    # output_lines[idx] = template.format(title=name, status='…', msg='', color="%s%s"%(Color.YELLOW, Color.BOLD), reset=Color.NORMAL)
    output_lines[idx] = template.format(title=name, status='░', msg='', color="%s%s"%(Color.YELLOW, Color.BOLD), reset=Color.NORMAL)

    error = []
    # while True:
    #     reads = [p.stdout.fileno(), p.stderr.fileno()]
    #     ret = select.select(reads, [], [])
    #
    #     for fd in ret[0]:
    #         if fd == p.stdout.fileno():
    #             #read = preprocess(p.stdout.readline())
    #             read = preprocess(p.stdout.read())
    #             output_lines[idx] = template.format(title=name, msg="%sWorking... %s%s" % (Color.YELLOW, Color.NORMAL, ":)"), color=Color.NORMAL, reset=Color.NORMAL)
    #
    #         elif fd == p.stderr.fileno():
    #             #read = preprocess(p.stderr.readline())
    #             read = preprocess(p.stderr.read())
    #             error.append(read.rstrip())
    #             #output_lines[idx] = template.format(title=name, msg="Error:" + read.split('\n')[0], color=Color.RED, reset=Color.NORMAL)
    #             output_lines[idx] = template.format(title=name, msg="%sWorking... %s%s" % (Color.YELLOW, Color.NORMAL, ":)"), color=Color.NORMAL, reset=Color.NORMAL)
    #     if p.poll() != None:
    #         break
    #
    #
    # #read = preprocess(p.stdout.readline())
    # read = preprocess(p.stdout.read())
    # output_lines[idx] = template.format(title=name, msg="%sDone... %s%s" % (Color.YELLOW, Color.NORMAL, ":)"), color=Color.NORMAL, reset=Color.NORMAL)
    #
    # #read = preprocess(p.stderr.readline())
    # read = preprocess(p.stderr.read())
    # error.append(read.rstrip())
    # output_lines[idx] = template.format(title=name, msg="%sDone... %s%s" % (Color.YELLOW, Color.NORMAL, ":)"), color=Color.NORMAL, reset=Color.NORMAL)

    p.communicate()
    p.wait()

    if p.returncode != 0:
        EXIT = True
        if len(error) > 0:
            output_lines[idx] = template.format(title=name, status="█", msg="%s Error (%d): stderr to follow...%s" % (Color.RED+Color.BOLD,p.returncode, Color.NORMAL), color=Color.RED, reset=Color.NORMAL)
        else:
            output_lines[idx] = template.format(title=name, status="█", msg="%s Error (%d)%s" % (Color.RED+Color.BOLD,p.returncode, Color.NORMAL), color=Color.RED, reset=Color.NORMAL)
    else:
        output_lines[idx] = template.format(title=name, status="█", msg="", color="%s%s"%(Color.GREEN, Color.BOLD), reset=Color.NORMAL)

    results[idx] = Result(name, cmd, p.returncode, "\n".join(error))


def run_tasks(tasks, title=None):
    length, offset = len(tasks), 0
    if title:
        offset = 1
    with output(output_type='list', initial_len=length+offset, interval=0) as output_lines:
        if title:
            output_lines[0] = TEMPLATE.format(title="{}{}{}".format(Color.BOLD,title,Color.NORMAL), status='░', msg='', color=Color.YELLOW, reset=Color.NORMAL)
        proc = []
        results = [None]*(length+offset)
        MAX_NAME_LEN = max([len(name) for name, cmd in tasks.items()])
        for idx, (name, cmd) in enumerate(tasks.items()):
            time.sleep(0.01)

            p = threading.Thread(target=exec_task, args=(output_lines, idx+offset, name, cmd, results, len(tasks)!=1, idx==len(tasks)-1))
            proc.append(p)
            p.start()
        [p.join() for p in proc]
        if title:
            output_lines[0] = TEMPLATE.format(title="{}{}{}".format(Color.BOLD,title,Color.NORMAL), status='█', msg='', color=Color.GREEN, reset=Color.NORMAL)

    for result in results:
        if result != None and result.returncode != 0:
            print "\n%s%sTask '%s' finished with error: %s%s" % (Color.BOLD,Color.RED, no_ansi(result.name.split('〔')[0]), result.returncode, Color.NORMAL)
            if result.stderr:
                print "%s%s%s" % (Color.RED, result.stderr.strip(), Color.NORMAL)

def process_task(options, bold_name=False):
    global MAX_NAME_LEN
    if isinstance(options, dict):
        if 'name' in options and 'cmd' in options:
            name, cmd = str(options['name']), options['cmd']
        elif 'cmd' in options:
            name, cmd = options['cmd'], options['cmd']
        else:
            raise RuntimeError("Task requires a name and cmd")

    if bold_name:
        name = "%s%s%s" % (Color.BOLD, name, Color.NORMAL)
    MAX_NAME_LEN = min(max(MAX_NAME_LEN, ansi_len(name) ), CAP_NAME_LEN)

    return name, cmd

def step_number_format(idx, length, name):
    return "%s%s〔%s/%s〕" % (name, Color.NORMAL+Color.PURPLE,idx+1, length)

def build_serial(idx, length, options):
    name, cmd = process_task(options, bold_name=False)
    return partial(run_tasks, {step_number_format(idx, length, name): cmd})

def build_parallel(idx, length, options):
    global INDENT
    INDENT = INDENT_LEN
    tasks = collections.OrderedDict()

    if 'title' not in options:
        raise RuntimeError('Parallel option requires title option. Given: %s' % repr(options))
    title = options['title']

    if 'tasks' not in options:
        raise RuntimeError('Parallel option requires tasks. Given: %s' % repr(options))
    for task_options in options['tasks']:
        name, cmd = process_task(task_options)
        tasks[name] = cmd

    return partial(run_tasks, tasks, title=step_number_format(idx, length, title))

def builder(task_yml_obj):
    ret = []
    length = len(task_yml_obj)
    for idx, item in enumerate(task_yml_obj):
        if 'cmd' in item.keys():
            ret.append(build_serial(idx, length, item))
        elif 'parallel' in item.keys():
            ret.append(build_parallel(idx, length, item['parallel']))
        else:
            raise RuntimeError("Unknown config item: %s" % repr(item))
    return ret

def main():
    version = 'bashful %s' % __version__
    args = docopt(__doc__, version=version)

    task_yml_obj = yaml.load(open(args['<ymlfile>'],'r').read())

    # import signal
    # import sys
    # def signal_handler(signal, frame):
    #     sys.exit(1)
    # signal.signal(signal.SIGINT, signal_handler)

    for func in builder(task_yml_obj):
        if EXIT:
            sys.exit(1)
        func()


if __name__ == '__main__':
    main()
