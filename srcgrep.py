#!/usr/bin/env python3

import io
import json
import subprocess


def src_search(query):
    proc = subprocess.Popen(['src', 'search', '-json', '-stream', query], stdout=subprocess.PIPE)
    for line in io.TextIOWrapper(proc.stdout, encoding="utf-8"):
        result = json.loads(line)
        if result.get('type') != 'content':
            continue
        path = result['path']
        for m in result['chunkMatches']:
            start = m['ranges'][0]['start']
            print('{}\00{}:{}'.format(path, start['line'] + 1, m['content']))

if __name__ == '__main__':
    import sys
    src_search(' '.join(sys.argv[1:]))
