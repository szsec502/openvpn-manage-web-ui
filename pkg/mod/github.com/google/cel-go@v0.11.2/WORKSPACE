workspace(name = "cel_go")

load("@bazel_tools//tools/build_defs/repo:http.bzl", "http_archive")

http_archive(
    name = "io_bazel_rules_go",
    sha256 = "207fad3e6689135c5d8713e5a17ba9d1290238f47b9ba545b63d9303406209c6",
    urls = [
        "https://mirror.bazel.build/github.com/bazelbuild/rules_go/releases/download/v0.24.7/rules_go-v0.24.7.tar.gz",
        "https://github.com/bazelbuild/rules_go/releases/download/v0.24.7/rules_go-v0.24.7.tar.gz",
    ],
)

http_archive(
    name = "bazel_gazelle",
    sha256 = "b85f48fa105c4403326e9525ad2b2cc437babaa6e15a3fc0b1dbab0ab064bc7c",
    urls = [
        "https://mirror.bazel.build/github.com/bazelbuild/bazel-gazelle/releases/download/v0.22.2/bazel-gazelle-v0.22.2.tar.gz",
        "https://github.com/bazelbuild/bazel-gazelle/releases/download/v0.21.2/bazel-gazelle-v0.22.2.tar.gz",
    ],
)

# Latest version as of 10/27/2020
http_archive(
    name = "rules_proto",
    sha256 = "8e7d59a5b12b233be5652e3d29f42fba01c7cbab09f6b3a8d0a57ed6d1e9a0da",
    strip_prefix = "rules_proto-7e4afce6fe62dbff0a4a03450143146f9f2d7488",
    urls = [
        "https://mirror.bazel.build/github.com/bazelbuild/rules_proto/archive/7e4afce6fe62dbff0a4a03450143146f9f2d7488.tar.gz",
        "https://github.com/bazelbuild/rules_proto/archive/7e4afce6fe62dbff0a4a03450143146f9f2d7488.tar.gz",
    ],
)

# Latest version of googleapis as of 7/27/2020
http_archive(
    name = "com_google_googleapis",
    sha256 = "1f742f6cafe616fe73302db010e0b7ee6579cb1ce06010427b7d0995cbd80ce4",
    strip_prefix = "googleapis-6a813acf535e4746fa4a135ce23547bb6425c26d",
    urls = [
        "https://github.com/googleapis/googleapis/archive/6a813acf535e4746fa4a135ce23547bb6425c26d.tar.gz",
    ],
)

# protobuf
http_archive(
    name = "com_google_protobuf",
    sha256 = "1c744a6a1f2c901e68c5521bc275e22bdc66256eeb605c2781923365b7087e5f",
    strip_prefix = "protobuf-3.13.0",
    urls = ["https://github.com/protocolbuffers/protobuf/archive/v3.13.0.zip"],
)

load("@io_bazel_rules_go//go:deps.bzl", "go_register_toolchains", "go_rules_dependencies")
load("@bazel_gazelle//:deps.bzl", "gazelle_dependencies", "go_repository")
load("@com_google_googleapis//:repository_rules.bzl", "switched_rules_by_language")
load("@rules_proto//proto:repositories.bzl", "rules_proto_dependencies", "rules_proto_toolchains")
load("@com_google_protobuf//:protobuf_deps.bzl", "protobuf_deps")

switched_rules_by_language(
    name = "com_google_googleapis_imports",
    go = True,
)

# Do *not* call *_dependencies(), etc, yet.  See comment at the end.

go_repository(
    name = "org_golang_google_protobuf",
    build_file_proto_mode = "disable_global",
    importpath = "google.golang.org/protobuf",
    tag = "v1.25.0",
)

# Generated Google APIs protos for Golang 11/16/2020
go_repository(
    name = "org_golang_google_genproto",
    build_file_proto_mode = "disable_global",
    commit = "62d171c70ae133bd47722027b62f8820407cf744",
    importpath = "google.golang.org/genproto",
)

# gRPC deps for v1.33.2 (including x/text and x/net)
go_repository(
    name = "org_golang_google_grpc",
    build_file_proto_mode = "disable_global",
    importpath = "google.golang.org/grpc",
    tag = "v1.33.2",
)

go_repository(
    name = "org_golang_x_net",
    importpath = "golang.org/x/net",
    sum = "h1:oWX7TPOiFAMXLq8o0ikBYfCJVlRHBcsciT5bXOrH628=",
    version = "v0.0.0-20190311183353-d8887717615a",
)

go_repository(
    name = "org_golang_x_text",
    importpath = "golang.org/x/text",
    sum = "h1:tW2bmiBqwgJj/UpqtC8EpXEZVYOwU0yG4iWbprSVAcs=",
    version = "v0.3.2",
)

# Antlr deps to pickup golang concurrency fixes 4/30/2020
go_repository(
    name = "com_github_antlr",
    commit = "621b933c7a7f01c67ae9de15103151fa0f9d6d90",
    importpath = "github.com/antlr/antlr4",
)

# CEL Spec deps
go_repository(
    name = "com_google_cel_spec",
    commit = "b11d0c7144434ceec0fa602ad8391c08f4a591a9",
    importpath = "github.com/google/cel-spec",
)

# strcase deps
go_repository(
    name = "com_github_stoewer_go_strcase",
    importpath = "github.com/stoewer/go-strcase",
    sum = "h1:Z2iHWqGXH00XYgqDmNgQbIBxf3wrNq0F3feEy0ainaU=",
    version = "v1.2.0",
)

# Run the dependencies at the end.  These will silently try to import some
# of the above repositories but at different versions, so ours must come first.
go_rules_dependencies()

go_register_toolchains()

gazelle_dependencies()

rules_proto_dependencies()

rules_proto_toolchains()

protobuf_deps()
