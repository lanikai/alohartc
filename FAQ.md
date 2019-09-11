# FAQ

---

**Q: I cannot get go modules to read from github.com/lanikai/oahu:**

    go: github.com/lanikai/oahu/api@v0.0.0-20190703205954-e5008c1038bd: invalid version: git fetch -f https://github.com/lanikai/oahu refs/heads/*:refs/heads/* refs/tags/*:refs/tags/* in /Users/chris/go/pkg/mod/cache/vcs/b31fd0c73a8293214deae52b6714931ccf64deb00ffbabb9b5dbea86f52b8fcf: exit status 128:
    	fatal: could not read Username for 'https://github.com': terminal prompts disabled
    make[1]: *** [generate] Error 1
    make: *** [generate] Error 2

A: To fix, run:

    git config --global url."ssh://git@github.com/".insteadOf "https://github.com/"

---

**Q: How to I enable logging?**

A: Use the `LOGLEVEL` environment variable to specify the internal component and severity. For exampel:

    LOGLEVEL=ice=debug ./alohartcd

---
