loco
====

CLI utility to work with DCC equpied locomotives and wagons.


Working with CV's
-----------------

### Retrieving multiple CVs

```bash
$ loco cv get cv1,cv2
cv1=17
cv2=2
```

### Specyfing a range of CVs

```bash
$ loco cv get cv1-cv255
cv1=17
cv2=2
# ...
cv255=5
```

### Retrieving a single CV

```bash
$ loco cv get cv1
17
```

### Specyfing a track type

```bash
# -t: track type, could be pom or prog
# -l: locomotive address
$ loco cv get cv2 -t pom -l 17
5

# when the "-l" is not specified the programming track is automatically chosen
$ loco cv get cv2
5
```

