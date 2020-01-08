package log

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/happyhakka/qc/util"

	"github.com/natefinch/lumberjack"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	Logger      *zap.Logger
	Log         *zap.SugaredLogger
	atomicLevel zap.AtomicLevel
)

type LoggerOption struct {
	FilePath   string //日志文件路径
	Level      string //日志级别: debug,info,warn,error,panic,fatal
	MaxSize    int32  //每个日志文件保存的最大尺寸 单位：M
	MaxBackups int32  //日志文件最多保存多少个备份
	MaxAge     int32  //文件最多保存多少天
	Compress   bool   //是否压缩
	Console    bool   //是否打印到屏幕上，缺少为不打印false
}

func NewLogger(opt *LoggerOption) *zap.Logger {

	var level zapcore.Level
	level.Set(strings.ToLower(opt.Level))
	// switch strings.ToLower(opt.Level) {
	// case "debug":
	// 	level = zap.DebugLevel
	// case "info":
	// 	level = zap.InfoLevel
	// case "warn":
	// 	level = zap.WarnLevel
	// case "error":
	// 	level = zap.ErrorLevel
	// case "panic":
	// 	level = zap.PanicLevel
	// case "fatal":
	// 	level = zap.FatalLevel
	// default:
	// 	level = zap.DebugLevel
	// }

	return buildLogger(opt.FilePath, level, int(opt.MaxSize), int(opt.MaxBackups), int(opt.MaxAge), opt.Compress, opt.Console)
}

//   NewLogger 获取日志
//   filePath 日志文件路径
//   level 日志级别
//   maxSize 每个日志文件保存的最大尺寸 单位：M
//   maxBackups 日志文件最多保存多少个备份
//   maxAge 文件最多保存多少天
//   compress 是否压缩
//   serviceName 服务名
func buildLogger(filePath string, level zapcore.Level, maxSize int, maxBackups int, maxAge int, compress bool, console bool) *zap.Logger {
	core := newCore(filePath, level, maxSize, maxBackups, maxAge, compress, console)
	return zap.New(core, zap.AddCaller(), zap.Development())
}

//  newCore 构造日志模块
func newCore(filePath string, level zapcore.Level, maxSize int, maxBackups int, maxAge int, compress bool, console bool) zapcore.Core {
	//日志文件路径配置2
	hook := lumberjack.Logger{
		Filename:   filePath,   // 日志文件路径
		MaxSize:    maxSize,    // 每个日志文件保存的最大尺寸 单位：M
		MaxBackups: maxBackups, // 日志文件最多保存多少个备份
		MaxAge:     maxAge,     // 文件最多保存多少天
		Compress:   compress,   // 是否压缩
	}
	// 设置日志级别
	atomicLevel = zap.NewAtomicLevel()
	atomicLevel.SetLevel(level)

	//公用编码器
	encoderConfig := zapcore.EncoderConfig{
		TimeKey:        "t",
		LevelKey:       "l",
		NameKey:        "n",
		CallerKey:      "c",
		MessageKey:     "m",
		StacktraceKey:  "s",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.LowercaseLevelEncoder,  // 小写编码器
		EncodeTime:     TcLoggerTimeEncoder,            //zapcore.ISO8601TimeEncoder,     // ISO8601 UTC 时间格式
		EncodeDuration: zapcore.SecondsDurationEncoder, //
		EncodeCaller:   zapcore.ShortCallerEncoder,     //zapcore.FullCallerEncoder,      // 全路径编码器
		EncodeName:     zapcore.FullNameEncoder,
	}

	if console {
		return zapcore.NewCore(
			zapcore.NewJSONEncoder(encoderConfig),
			zapcore.NewMultiWriteSyncer(zapcore.AddSync(os.Stdout), zapcore.AddSync(&hook)), // 打印到控制台和文件
			atomicLevel, // 日志级别
		)
	} else {
		return zapcore.NewCore(
			zapcore.NewJSONEncoder(encoderConfig),
			zapcore.NewMultiWriteSyncer(zapcore.AddSync(&hook)), // 打印到控制台和文件
			atomicLevel, // 日志级别
		)
	}
}

func SetLogLevel(strLevel string) {
	var level zapcore.Level
	level.Set(strings.ToLower(strLevel))

	if Logger != nil {
		atomicLevel.SetLevel(level)
	}

	if Log != nil {
		atomicLevel.SetLevel(level)
	}
}

// TcLoggerTimeEncoder serializes a time.Time to an ISO8601-formatted string
// with millisecond precision.
func TcLoggerTimeEncoder(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
	enc.AppendString(t.Format("2006-01-02 15:04:05.000"))
}

func InitLog(logConfigFile string) (*zap.Logger, error) {
	if len(logConfigFile) <= 0 {
		return nil, fmt.Errorf("log config file no found!")
	}

	opt := &LoggerOption{}
	jsonParser := util.NewJsonParser()
	err := jsonParser.Load(logConfigFile, opt)
	if err != nil {
		return nil, err
	}

	Logger = NewLogger(opt)
	Logger.Info("log init ok.", zap.String("LogLevel", opt.Level))
	Log := Logger.Sugar()
	Log.Infof("log init ok level:%v", opt.Level)

	return Logger, nil
}
