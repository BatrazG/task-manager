package middleware

import "net/http"

// statusRecorder позволяет логгеру узнать статус и размер ответа.
// Это важный кусок "готовности к эксплуатации".
type statusRecorder struct {
	http.ResponseWriter
	status int
	bytes  int
}

func newStatusRecorder(w http.ResponseWriter) *statusRecorder {
	return &statusRecorder{ResponseWriter: w, status: http.StatusOK}
}

func (r *statusRecorder) WriteHeader(statusCode int) {
	r.status = statusCode
	r.ResponseWriter.WriteHeader(statusCode)
}

func (r *statusRecorder) Write(p []byte) (int, error) {
	// Если handler не вызвал WriteHeader явно -- статус считается 200.
	n, err := r.ResponseWriter.Write(p)
	r.bytes += n
	return n, err
}
