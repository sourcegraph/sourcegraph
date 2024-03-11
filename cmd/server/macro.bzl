load("@rules_pkg//:pkg.bzl", "pkg_tar")

def get_last_segment(path):
    segments = path.split("/")
    last_segment = segments[-1]

    s = last_segment.split(":")
    if len(s) == 1:
        return last_segment
    else:
        return s[-1]

def container_dependencies(targets):
    for target in targets:
        name = get_last_segment(target)

        pkg_tar(
            name = "tar_{}".format(name),
            srcs = [target],
            remap_paths = {"/{}".format(name): "/usr/local/bin/{}".format(name)},
            modes = {"/usr/local/bin/{}".format(name): "0555"},
        )

def dependencies_tars(targets):
    tars = []
    for target in targets:
        name = get_last_segment(target)
        tars.append(":tar_{}".format(name))

    return tars
