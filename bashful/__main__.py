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
from frozendict import frozendict
from enum import Enum
import collections
import subprocess
import threading
import logging
import signal
import select
import shlex
import time
import yaml
import sys
import io

import six
if six.PY2:
    from backports.shutil_get_terminal_size import get_terminal_size
else:
    from shutil import get_terminal_size

from bashful.version import __version__
from bashful.reprint import output, ansi_len, preprocess, no_ansi


SUPRESS_OUT = True
LOGGING = False
TEMPLATE               = " {color}{status}{reset} {title:25s} {msg}"
PARALLEL_TEMPLATE      = " {color}{status}{reset}  ├─ {title:25s} {msg}"
LAST_PARALLEL_TEMPLATE = " {color}{status}{reset}  └─ {title:25s} {msg}"
ERROR_TEMPLATE         = " {color}{status}{reset} {msg}"


EXIT = False

Task = collections.namedtuple("Task", "name cmd options")
Result = collections.namedtuple("Result", "name cmd returncode stderr stdout")


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
                return template.format(title=title, status="█", msg="%s Failed! (stderr to follow...)%s" % (Color.RED+Color.BOLD, Color.NORMAL), color=Color.RED, reset=Color.NORMAL)
            return template.format(title=title, status="█", msg="%s Failed! %s" % (Color.RED+Color.BOLD, Color.NORMAL), color=Color.RED, reset=Color.NORMAL)
        return template.format(title=title, status="█", msg="", color="%s%s"%(Color.GREEN, Color.BOLD), reset=Color.NORMAL)

    output = ''
    remaining_space = 0
    if stdout or stderr:
        cols, rows = get_terminal_size()
        dummy = template.format(title=title, status='x', msg="", color=Color.NORMAL, reset=Color.NORMAL)
        dummy_no_ansi = no_ansi(unicode(dummy, 'utf-8'))
        remaining_space = max((cols-5) - len(dummy_no_ansi), 0)

    if stdout:
        output = " "+no_ansi(stdout)
    if stderr:
        output = " "+no_ansi(stderr)
    if len(output) > remaining_space:
        output = no_ansi(output[:remaining_space-3] + "...")

    if LOGGING:
        logging.info(output.strip())
    output = Color.PURPLE + output + Color.NORMAL

    # is still running
    if status in (TaskStatus.init, TaskStatus.running):
        #print repr(stdout)
        return template.format(title=title, status='░', msg=output, color=Color.YELLOW, reset=Color.NORMAL)
    elif status in (TaskStatus.successful, ):
        return template.format(title=title, status='█', msg=output, color=Color.GREEN, reset=Color.NORMAL)
    return template.format(title=title, status='█', msg="", color=Color.RED, reset=Color.NORMAL)

def format_error(output, extra=None):
    ret = []
    lines = output.split('\n')
    for idx, line in enumerate(lines):
        line = "%s%s%s" % (Color.RED+Color.BOLD, line, Color.NORMAL)
        if idx == 0:
            ret.append( ERROR_TEMPLATE.format(status="█➜", msg=line, color=Color.RED, reset=Color.NORMAL) )
        else:
            ret.append( ERROR_TEMPLATE.format(status="░   ", msg=line, color=Color.RED, reset=Color.NORMAL) )

    if extra:
        lines = extra.split('\n')
        for idx, line in enumerate(lines):
            line = "%s%s%s" % (Color.RED, line, Color.NORMAL)
            ret.append( ERROR_TEMPLATE.format(status="░   ", msg=line, color=Color.RED, reset=Color.NORMAL) )


    return "\n".join(ret)
    # return output

LIMIT = 500

def exec_task(out_proxy, idx, task, results, is_parallel=False, is_last=False, name_suffix=''):
    global EXIT

    p = subprocess.Popen(shlex.split(task.cmd), stdout=subprocess.PIPE, stderr=subprocess.PIPE)
    out_proxy[idx] = format_step(is_parallel=is_parallel, status=TaskStatus.running, title=task.name+name_suffix, returncode=None, stderr=None, stdout=None, is_last=is_last)

    # This needs to happen, however, when reading you shouldn't depend on there being line breaks at reasonable times...
    # in fact, don't depend on any!


    stdout_audit, stderr_audit = collections.deque(maxlen=100), []
    stdout, stderr = [],[]
    if not SUPRESS_OUT:
        while True:
            reads = [p.stdout.fileno(), p.stderr.fileno()]
            ret = select.select(reads, [], [])

            for fd in ret[0]:
                if fd == p.stdout.fileno():
                    stdout_chr = p.stdout.read(1)

                    if stdout_chr != None and len(stdout_chr) > 0:
                        stdout.append(stdout_chr)

                        if stdout_chr == "\n" or len(stdout) > LIMIT:
                            line = no_ansi(preprocess("".join(stdout)))
                            stdout_audit.append(line)
                            out_proxy[idx] = format_step(is_parallel=is_parallel, status=TaskStatus.running, title=task.name+name_suffix, returncode=None, stderr=None, stdout=line, is_last=is_last)
                            stdout = []

                elif fd == p.stderr.fileno():
                    stderr_chr = p.stderr.read(1)

                    if stderr_chr != None and len(stderr_chr) > 0:
                        stderr.append(stderr_chr)

                        if stderr_chr == "\n" or len(stderr) > LIMIT:

                            out_proxy[idx] = format_step(is_parallel=is_parallel, status=TaskStatus.running, title=task.name+name_suffix, returncode=None, stderr=no_ansi(preprocess("".join(stderr))), stdout=None, is_last=is_last)
                            stderr_audit.append("".join(stderr))
                            stderr = []

            if p.poll() != None:
                break

        read = no_ansi(preprocess(p.stdout.read()))
        out_proxy[idx] = format_step(is_parallel=is_parallel, status=TaskStatus.running, title=task.name+name_suffix, returncode=None, stderr=None, stdout=read, is_last=is_last)

        read = no_ansi(preprocess(p.stderr.read()))
        stderr_audit.append(read.rstrip())
        out_proxy[idx] = format_step(is_parallel=is_parallel, status=TaskStatus.running, title=task.name+name_suffix, returncode=None, stderr=read, stdout=None, is_last=is_last)

    else:
        out, err = p.communicate()
        p.wait()
        stderr_audit = [err]
        stdout_audit = [out]

    status = TaskStatus.successful
    if p.returncode != 0:
        status = TaskStatus.failed
        if ('stop_on_failure' in task.options and task.options['stop_on_failure']) or ('stop_on_failure' not in task.options):
            EXIT = True

    out_proxy[idx] = format_step(is_parallel=is_parallel, status=status, title=task.name+name_suffix, returncode=p.returncode, stderr=stderr_audit, stdout=None, is_last=is_last)
    results[idx] = Result(task.name, task.cmd, p.returncode, "\n".join(stderr_audit), "\n".join(stdout_audit))


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
            for idx, (name, task) in enumerate(self.tasks.items()):
                time.sleep(0.01)

                name_suffix = ''
                if not self.is_parallel:
                    name_suffix = self.formatted_step_num

                p = threading.Thread(target=exec_task, args=(out_proxy, idx+offset, task, results, len(self.tasks)>1, idx==len(self.tasks)-1, name_suffix))
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

                error_msg = "Error %d: task '%s' failed with error (returncode:%s)" % (err_idx, no_ansi(result.name.split('〔')[0]), result.returncode)

                extra = ""
                if len(result.stderr) > 0:
                    extra += Color.BOLD+Color.RED+"Last stderr:\n"+Color.NORMAL+Color.RED+result.stderr
                if len(result.stdout) > 0:
                    extra += Color.BOLD+Color.RED+"Last stdout:\n"+Color.NORMAL+Color.RED+result.stdout
                print(format_error(error_msg, extra=extra))

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
        if 'tasks' not in yaml_obj:
            raise RuntimeError("Require tasks option at root")

        self.num_tasks = len(yaml_obj['tasks'])

        for idx, item in enumerate(yaml_obj['tasks']):
            if 'cmd' in item.keys():
                self.tasks.append(self._build_serial(idx, item))
            elif 'parallel' in item.keys():
                self.tasks.append(self._build_parallel(idx, item['parallel']))
            else:
                raise RuntimeError("Unknown config item: %s" % repr(item))

    def _build_serial(self, idx, options):
        name, cmd, remaining_options = self._process_task(options, bold_name=False)
        tasks = {name: Task(name, cmd, remaining_options)}
        return TaskSet(tasks=tasks, title=name, num=idx+1, total=self.num_tasks)

    def _build_parallel(self, idx, options):
        tasks = collections.OrderedDict()

        if 'title' not in options:
            raise RuntimeError('Parallel option requires title option. Given: %s' % repr(options))
        title = options['title']

        if 'tasks' not in options:
            raise RuntimeError('Parallel option requires tasks. Given: %s' % repr(options))

        for task_options in options['tasks']:
            name, cmd, remaining_options = self._process_task(task_options)
            tasks[name] = Task(name, cmd, remaining_options)

        return TaskSet(tasks, title=title, num=idx+1, total=self.num_tasks)

    def _process_task(self, options, bold_name=False):
        if isinstance(options, dict):
            if 'name' in options and 'cmd' in options:
                name, cmd = str(options['name']), options['cmd']
                del options['name']
                del options['cmd']
            elif 'cmd' in options:
                name, cmd = options['cmd'], options['cmd']
                del options['cmd']
            else:
                raise RuntimeError("Task requires a name and cmd")

        if bold_name:
            name = "%s%s%s" % (Color.BOLD, name, Color.NORMAL)

        return name, cmd, frozendict(options)

    def execute(self):
        self._parse()
        for task_set in self.tasks:
            if EXIT:
                print("Aborted!")
                sys.exit(1)
            task_set.execute()


def main():
    version = 'bashful %s' % __version__
    args = docopt(__doc__, version=version)

    if LOGGING:
        logging.basicConfig(filename="build.log", level=logging.INFO)
    prog = Program(args['<ymlfile>'])
    prog.execute()


if __name__ == '__main__':
    main()
