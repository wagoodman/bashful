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


TEMPLATE               = " {color}{status}{reset} {title} {msg}"
PARALLEL_TEMPLATE      = " {color}{status}{reset}  ├─ {title} {msg}"
LAST_PARALLEL_TEMPLATE = " {color}{status}{reset}  └─ {title} {msg}"

EXIT = False

Result = collections.namedtuple("Result", "name cmd returncode stderr")


class TaskStatus(Enum):
    init = 0
    running = 1
    failed = 2
    successful = 3

class Color(Enum):
    PURPLE = '\033[95m'
    BLUE = '\033[94m'
    GREEN = '\033[92m'
    YELLOW = '\033[93m'
    RED = '\033[91m'
    NORMAL = '\033[0m'
    BOLD = '\033[1m'
    UNDERLINE = '\033[4m'
    INVERSE = '\033[7m'


def format_step(is_parallel, status, title, returncode=None, stderr=None, stdout=None, is_last=False):
    if is_parallel and is_last:
        template = LAST_PARALLEL_TEMPLATE
    elif is_parallel:
        template = PARALLEL_TEMPLATE
    else:
        template = TEMPLATE

    # has exited...
    if returncode != None:
        if returncode != 0:
            if stderr != None and len(stderr) > 0:
                return template.format(title=title, status="█", msg="%s Error (%d): stderr to follow...%s" % (Color.RED+Color.BOLD, returncode, Color.NORMAL), color=Color.RED, reset=Color.NORMAL)
            return template.format(title=title, status="█", msg="%s Error (%d)%s" % (Color.RED+Color.BOLD, returncode, Color.NORMAL), color=Color.RED, reset=Color.NORMAL)
        return template.format(title=title, status="█", msg="", color="%s%s"%(Color.GREEN, Color.BOLD), reset=Color.NORMAL)

    # is still running
    if status in (TaskStatus.init, TaskStatus.running):
        return template.format(title=title, status='░', msg='', color=Color.YELLOW, reset=Color.NORMAL)
    elif status in (TaskStatus.successful, ):
        return template.format(title=title, status='█', msg='', color=Color.GREEN, reset=Color.NORMAL)
    return template.format(title=title, status='?', msg='', color=Color.RED, reset=Color.NORMAL)


def exec_task(out_proxy, idx, name, cmd, results, is_parallel=False, is_last=False):
    global EXIT

    error = []
    p = subprocess.Popen(shlex.split(cmd), stdout=subprocess.PIPE, stderr=subprocess.PIPE)
    out_proxy[idx] = format_step(is_parallel=is_parallel, status=TaskStatus.running, title=name, returncode=None, stderr=None, stdout=None, is_last=is_last)

    # This needs to happen, however, when reading you shouldn't depend on there being line breaks at reasonable times...
    # in fact, don't depend on any!

    # while True:
    #     reads = [p.stdout.fileno(), p.stderr.fileno()]
    #     ret = select.select(reads, [], [])
    #
    #     for fd in ret[0]:
    #         if fd == p.stdout.fileno():
    #             #read = preprocess(p.stdout.readline())
    #             read = preprocess(p.stdout.read())
    #             out_proxy[idx] = template.format(title=name, msg="%sWorking... %s%s" % (Color.YELLOW, Color.NORMAL, ":)"), color=Color.NORMAL, reset=Color.NORMAL)
    #
    #         elif fd == p.stderr.fileno():
    #             #read = preprocess(p.stderr.readline())
    #             read = preprocess(p.stderr.read())
    #             error.append(read.rstrip())
    #             #out_proxy[idx] = template.format(title=name, msg="Error:" + read.split('\n')[0], color=Color.RED, reset=Color.NORMAL)
    #             out_proxy[idx] = template.format(title=name, msg="%sWorking... %s%s" % (Color.YELLOW, Color.NORMAL, ":)"), color=Color.NORMAL, reset=Color.NORMAL)
    #     if p.poll() != None:
    #         break
    #
    #
    # #read = preprocess(p.stdout.readline())
    # read = preprocess(p.stdout.read())
    # out_proxy[idx] = template.format(title=name, msg="%sDone... %s%s" % (Color.YELLOW, Color.NORMAL, ":)"), color=Color.NORMAL, reset=Color.NORMAL)
    #
    # #read = preprocess(p.stderr.readline())
    # read = preprocess(p.stderr.read())
    # error.append(read.rstrip())
    # out_proxy[idx] = template.format(title=name, msg="%sDone... %s%s" % (Color.YELLOW, Color.NORMAL, ":)"), color=Color.NORMAL, reset=Color.NORMAL)

    p.communicate()
    p.wait()

    status = TaskStatus.successful
    if p.returncode != 0:
        status = TaskStatus.failed
        EXIT = True

    out_proxy[idx] = format_step(is_parallel=is_parallel, status=status, title=name, returncode=p.returncode, stderr=error, stdout=None, is_last=is_last)
    results[idx] = Result(name, cmd, p.returncode, "\n".join(error))


class TaskSet:
    def __init__(self, tasks, title, num, total):
        # Todo: base this on a set of task definitinos that has name:cmd:options 
        self.tasks = tasks
        self.num = num
        self.total = total
        self.title = title

    @property
    def formatted_title(self):
        title = Color.BOLD + self.title + Color.NORMAL
        return "{title}{step}".format(title=title, step=self.formatted_step_num)

    @property
    def formatted_step_num(self):
        return "%s〔%s/%s〕%s" % (Color.NORMAL+Color.PURPLE, self.num, self.total, Color.NORMAL)

    @property
    def is_parallel(self):
        return len(self.tasks) > 1

    def execute(self):
        offset = 0

        if self.is_parallel:
            offset = 1

        with output(output_type='list', initial_len=len(self.tasks)+offset, interval=0) as out_proxy:
            if self.is_parallel:
                out_proxy[0] = format_step(is_parallel=False, status=TaskStatus.init, title=self.formatted_title)

            proc = []
            results = [None]*(len(self.tasks)+offset)
            for idx, (name, cmd) in enumerate(self.tasks.items()):
                time.sleep(0.01)

                if not self.is_parallel:
                    name+=self.formatted_step_num

                p = threading.Thread(target=exec_task, args=(out_proxy, idx+offset, name, cmd, results, len(self.tasks)>1, idx==len(self.tasks)-1))
                proc.append(p)
                p.start()

            [p.join() for p in proc]

            status = TaskStatus.successful
            for result in results:
                if result != None and result.returncode != 0:
                    status = TaskStatus.failed

            if self.is_parallel:
                out_proxy[0] = format_step(is_parallel=False, status=status, title=self.formatted_title)

        err_idx = 0
        for result in results:
            if result != None and result.returncode != 0:
                err_idx += 1
                print "\n%sError %d: task '%s' failed with error (returncode:%s)%s" % (Color.BOLD+Color.RED, err_idx, no_ansi(result.name.split('〔')[0]), result.returncode, Color.NORMAL)
                if result.stderr:
                    print Color.RED + result.stderr.strip() + Color.NORMAL

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
        return TaskSet(tasks={name: cmd}, title=name, num=idx+1, total=self.num_tasks)

    def _build_parallel(self, idx, options):
        tasks = collections.OrderedDict()

        if 'title' not in options:
            raise RuntimeError('Parallel option requires title option. Given: %s' % repr(options))
        title = options['title']

        if 'tasks' not in options:
            raise RuntimeError('Parallel option requires tasks. Given: %s' % repr(options))

        for task_options in options['tasks']:
            name, cmd = self._process_task(task_options)
            tasks[name] = cmd

        return TaskSet(tasks, title=title, num=idx+1, total=self.num_tasks)

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
