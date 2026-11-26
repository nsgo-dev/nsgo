package nsgotest

// ShaderSources holds each fragment backend's translated nsgoShader library
// function (MSL, HLSL, GLSL 3.30, and GLSL ES 3.00). GLSL and GLES include the
// platform version and precision headers; none include the platform host stage
// wrapper.
type ShaderSources struct {
	MSL  string
	HLSL string
	GLSL string
	GLES string
}
