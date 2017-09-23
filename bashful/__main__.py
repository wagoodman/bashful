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
from enum import Enum
import collections
import subprocess
import threading
import signal
import select
import shlex
import time
import yaml
import sys
import io

from bashful.version import __version__
from bashful.reprint import output, ansi_len, preprocess, no_ansi

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



def step_number_format(idx, length, name):
    return "%s%s〔%s/%s〕" % (name, Color.NORMAL+Color.PURPLE,idx+1, length)


class TaskSet:
    def __init__(self, tasks, title=None):
        self.tasks = tasks
        self.title = title

    def execute(self):
        offset = 0
        if self.title:
            offset = 1

        with output(output_type='list', initial_len=len(self.tasks)+offset, interval=0) as output_lines:
            if self.title:
                output_lines[0] = TEMPLATE.format(title="{}{}{}".format(Color.BOLD,self.title,Color.NORMAL), status='░', msg='', color=Color.YELLOW, reset=Color.NORMAL)
            proc = []
            results = [None]*(len(self.tasks)+offset)
            for idx, (name, cmd) in enumerate(self.tasks.items()):
                time.sleep(0.01)

                p = threading.Thread(target=exec_task, args=(output_lines, idx+offset, name, cmd, results, len(self.tasks)!=1, idx==len(self.tasks)-1))
                proc.append(p)
                p.start()

            [p.join() for p in proc]

            if self.title:
                output_lines[0] = TEMPLATE.format(title="{}{}{}".format(Color.BOLD,self.title,Color.NORMAL), status='█', msg='', color=Color.GREEN, reset=Color.NORMAL)

        for result in results:
            if result != None and result.returncode != 0:
                print "\n%s%sTask '%s' finished with error: %s%s" % (Color.BOLD,Color.RED, no_ansi(result.name.split('〔')[0]), result.returncode, Color.NORMAL)
                if result.stderr:
                    print "%s%s%s" % (Color.RED, result.stderr.strip(), Color.NORMAL)

class Program:

    def __init__(self, yaml_file):
        self.yaml_file = yaml_file
        self.tasks = []
        self.num_tasks = 0

        # in the future this will need to be handled such that output is not mangled
        # def signal_handler(signal, frame):
        #     sys.exit(0)
        # signal.signal(signal.SIGINT, signal_handler)

    def _parse(self):
        yaml_obj = yaml.load(open(self.yaml_file,'r').read())
        self.num_tasks = len(yaml_obj)

        for idx, item in enumerate(yaml_obj):
            if 'cmd' in item.keys():
                self.tasks.append(self._build_serial(idx, item))
            elif 'parallel' in item.keys():
                self.tasks.append(self._build_parallel(idx, item['parallel']))
            else:
                raise RuntimeError("Unknown config item: %s" % repr(item))

    def _build_serial(self, idx, options):
        name, cmd = self._process_task(options, bold_name=False)
        return TaskSet(tasks={step_number_format(idx, self.num_tasks, name): cmd})

    def _build_parallel(self, idx, options):
        global INDENT
        INDENT = INDENT_LEN
        tasks = collections.OrderedDict()

        if 'title' not in options:
            raise RuntimeError('Parallel option requires title option. Given: %s' % repr(options))
        title = options['title']

        if 'tasks' not in options:
            raise RuntimeError('Parallel option requires tasks. Given: %s' % repr(options))

        for task_options in options['tasks']:
            name, cmd = self._process_task(task_options)
            tasks[name] = cmd

        return TaskSet(tasks, title=step_number_format(idx, self.num_tasks, title))

    def _process_task(self, options, bold_name=False):
        if isinstance(options, dict):
            if 'name' in options and 'cmd' in options:
                name, cmd = str(options['name']), options['cmd']
            elif 'cmd' in options:
                name, cmd = options['cmd'], options['cmd']
            else:
                raise RuntimeError("Task requires a name and cmd")

        if bold_name:
            name = "%s%s%s" % (Color.BOLD, name, Color.NORMAL)

        return name, cmd

    def execute(self):
        self._parse()
        for task_set in self.tasks:
            if EXIT:
                sys.exit(1)
            task_set.execute()


def main():
    version = 'bashful %s' % __version__
    args = docopt(__doc__, version=version)

    prog = Program(args['<ymlfile>'])
    prog.execute()


if __name__ == '__main__':
    main()
