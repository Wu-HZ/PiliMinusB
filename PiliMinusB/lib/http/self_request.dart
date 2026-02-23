import 'package:PiliPlus/http/constants.dart';
import 'package:PiliPlus/utils/storage.dart';
import 'package:dio/dio.dart';
import 'package:flutter/foundation.dart' show kDebugMode;

/// HTTP client for PiliMinusB self-hosted server.
///
/// Mirrors [Request] but targets [HttpString.selfBaseUrl] and uses
/// JWT Bearer token for authentication instead of Bilibili cookies.
class SelfRequest {
  static final SelfRequest _instance = SelfRequest._internal();
  static late final Dio dio;

  factory SelfRequest() => _instance;

  /// JWT token stored locally. Set after login.
  static String? _token;

  static String? get token => _token;

  static void setToken(String? t) {
    _token = t;
    if (t != null) {
      GStorage.setting.put('selfToken', t);
    } else {
      GStorage.setting.delete('selfToken');
    }
  }

  static void loadToken() {
    _token = GStorage.setting.get('selfToken') as String?;
  }

  SelfRequest._internal() {
    BaseOptions options = BaseOptions(
      baseUrl: HttpString.selfBaseUrl,
      connectTimeout: const Duration(milliseconds: 10000),
      receiveTimeout: const Duration(milliseconds: 10000),
      headers: {
        'content-type': 'application/json',
      },
    );

    dio = Dio(options);

    // Interceptor: attach JWT token to every request
    dio.interceptors.add(InterceptorsWrapper(
      onRequest: (options, handler) {
        if (_token != null) {
          options.headers['Authorization'] = 'Bearer $_token';
        }
        handler.next(options);
      },
    ));

    if (kDebugMode) {
      dio.interceptors.add(
        LogInterceptor(
          request: false,
          requestHeader: false,
          responseHeader: false,
        ),
      );
    }

    dio
      ..transformer = BackgroundTransformer()
      ..options.validateStatus = (int? status) {
        return status != null && status >= 200 && status < 300;
      };
  }

  Future<Response> get<T>(
    String url, {
    Map<String, dynamic>? queryParameters,
    Options? options,
    CancelToken? cancelToken,
  }) async {
    try {
      return await dio.get<T>(
        url,
        queryParameters: queryParameters,
        options: options,
        cancelToken: cancelToken,
      );
    } on DioException catch (e) {
      return _errorResponse(e);
    }
  }

  Future<Response> post<T>(
    String url, {
    Object? data,
    Map<String, dynamic>? queryParameters,
    Options? options,
    CancelToken? cancelToken,
  }) async {
    try {
      return await dio.post<T>(
        url,
        data: data,
        queryParameters: queryParameters,
        options: options,
        cancelToken: cancelToken,
      );
    } on DioException catch (e) {
      return _errorResponse(e);
    }
  }

  /// Login to self-hosted server. Returns `{'status': true, 'msg': '...'}` on
  /// success or `{'status': false, 'msg': '...'}` on failure.
  ///
  /// Server response format: `{"code":0,"message":"success","data":{"token":"..."}}`
  static Future<Map<String, dynamic>> login(
      String username, String password) async {
    try {
      final resp = await dio.post('/auth/login', data: {
        'username': username,
        'password': password,
      });
      final body = resp.data;
      final inner = body is Map ? body['data'] : null;
      if (inner is Map && inner['token'] != null) {
        setToken(inner['token'] as String);
        return {'status': true, 'msg': '登录成功'};
      }
      return {'status': false, 'msg': body?['message'] ?? '登录失败'};
    } on DioException catch (e) {
      final msg = e.response?.data is Map
          ? (e.response!.data as Map)['message'] ?? e.message
          : e.message ?? '网络错误';
      return {'status': false, 'msg': msg};
    }
  }

  /// Register on self-hosted server.
  ///
  /// Server response format: `{"code":0,"message":"success","data":{"id":...,"username":"..."}}`
  /// Register does NOT return a token — user must login after registering.
  static Future<Map<String, dynamic>> register(
      String username, String password) async {
    try {
      final resp = await dio.post('/auth/register', data: {
        'username': username,
        'password': password,
      });
      final body = resp.data;
      return {
        'status': body is Map && body['code'] == 0,
        'msg': body?['message'] ?? '注册成功，请登录',
      };
    } on DioException catch (e) {
      final msg = e.response?.data is Map
          ? (e.response!.data as Map)['message'] ?? e.message
          : e.message ?? '网络错误';
      return {'status': false, 'msg': msg};
    }
  }

  Response _errorResponse(DioException e) {
    final message = e.response?.data is Map
        ? (e.response!.data as Map)['message'] ?? e.message
        : e.message ?? 'network error';
    return Response(
      data: {'code': -1, 'message': message},
      statusCode: e.response?.statusCode ?? -1,
      requestOptions: e.requestOptions,
    );
  }
}
