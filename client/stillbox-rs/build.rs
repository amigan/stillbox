fn main() {
    protobuf_codegen::Codegen::new()
        .cargo_out_dir("protos")
        .include("../../pkg/pb")
        .input("../../pkg/pb/stillbox.proto")
        .run_from_script();
}