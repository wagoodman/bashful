# bashful
because your bash script should be quiet and shy

...but for the meantime keep them loud-like, 'cause this is a pre-alpha-weekend-project-work-in-progress thing :)

... actually, this should be rewritten in golang (or something). It appears that large sys.stdout.write() calls followed shortly thereafter by sys.stdout.flush() calls ends up failing (ERRNO 35 on stdout). 

Example:

```bash
bashful example/bashful.yml
```
