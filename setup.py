import setuptools
import glob
import os

exec(open('./bashful/version.py').read())

setuptools.setup(
    name='bashful',
    version=__version__,
    url='https://github.com/wagoodman/bashful',
    license=__license__,
    author=__author__,
    author_email=__email__,
    description='Because your bash scripts should be quiet and shy (and not such a loudmouth)',
    packages=setuptools.find_packages(),
    zip_safe=False,
    include_package_data=True,
    install_requires=['PyYAML',
                      'docopt',
                      'backports.shutil_get_terminal_size',
                      'six',
                      ],
    platforms='linux',
    keywords=['bash'],
    # latest from https://pypi.python.org/pypi?%3Aaction=list_classifiers
    classifiers = [
        'Development Status :: 4 - Beta',
        'Environment :: Console',
        'Intended Audience :: Developers',
        'License :: OSI Approved :: MIT License',
        'Natural Language :: English',
        'Operating System :: POSIX :: Linux',
        'Programming Language :: Python',
        'Programming Language :: Python :: 2',
        'Programming Language :: Python :: 3',
        'Programming Language :: Python :: Implementation :: CPython',
        'Topic :: Software Development :: Libraries :: Python Modules',
        'Topic :: System :: Systems Administration',
        'Topic :: System :: System Shells',
        'Topic :: Terminals',
        'Topic :: Utilities',
        ],
    entry_points={
        'console_scripts': [
            'bashful = bashful.__main__:main'
        ]
    },
)
