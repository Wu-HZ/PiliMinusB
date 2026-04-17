import 'dart:async';
import 'dart:convert';
import 'dart:io';
import 'dart:typed_data';

import 'package:PiliPlus/common/constants.dart';
import 'package:PiliPlus/http/api.dart';
import 'package:PiliPlus/http/constants.dart';
import 'package:PiliPlus/http/init.dart';
import 'package:PiliPlus/http/loading_state.dart';
import 'package:PiliPlus/http/login.dart';
import 'package:PiliPlus/http/self_request.dart';
import 'package:PiliPlus/http/ua_type.dart';
import 'package:PiliPlus/models/common/account_type.dart';
import 'package:PiliPlus/models/common/video/video_type.dart';
import 'package:PiliPlus/models/home/rcmd/result.dart';
import 'package:PiliPlus/models/model_hot_video_item.dart';
import 'package:PiliPlus/models/model_rec_video_item.dart';
import 'package:PiliPlus/models/pgc_lcf.dart';
import 'package:PiliPlus/models/video/play/url.dart';
import 'package:PiliPlus/models_new/pgc/pgc_rank/pgc_rank_item_model.dart';
import 'package:PiliPlus/models_new/popular/popular_precious/data.dart';
import 'package:PiliPlus/models_new/popular/popular_series_list/list.dart';
import 'package:PiliPlus/models_new/popular/popular_series_one/data.dart';
import 'package:PiliPlus/models_new/triple/pgc_triple.dart';
import 'package:PiliPlus/models_new/triple/ugc_triple.dart';
import 'package:PiliPlus/models_new/video/video_ai_conclusion/data.dart';
import 'package:PiliPlus/models_new/video/video_detail/data.dart';
import 'package:PiliPlus/models_new/video/video_detail/video_detail_response.dart';
import 'package:PiliPlus/models_new/video/video_note_list/data.dart';
import 'package:PiliPlus/models_new/video/video_play_info/data.dart';
import 'package:PiliPlus/models_new/video/video_relation/data.dart';
import 'package:PiliPlus/utils/accounts.dart';
import 'package:PiliPlus/utils/accounts/account.dart';
import 'package:PiliPlus/utils/app_sign.dart';
import 'package:PiliPlus/utils/extension/string_ext.dart';
import 'package:PiliPlus/utils/global_data.dart';
import 'package:PiliPlus/utils/id_utils.dart';
import 'package:PiliPlus/utils/recommend_filter.dart';
import 'package:PiliPlus/utils/storage_pref.dart';
import 'package:PiliPlus/utils/wbi_sign.dart';
import 'package:dio/dio.dart';
import 'package:flutter/foundation.dart' show compute;

/// view层根据 status 判断渲染逻辑
abstract final class VideoHttp {
  static RegExp zoneRegExp = RegExp(Pref.banWordForZone, caseSensitive: false);
  static bool enableFilter = zoneRegExp.pattern.isNotEmpty;
  static final Dio _saucDio =
      Dio(
          BaseOptions(
            baseUrl: HttpString.saucBaseUrl,
            connectTimeout: const Duration(seconds: 15),
            sendTimeout: const Duration(minutes: 10),
            receiveTimeout: const Duration(hours: 2),
          ),
        )
        ..options.validateStatus = (int? status) {
          return status != null && status >= 200 && status < 300;
        };

  // 首页推荐视频
  static Future<LoadingState<List<RecVideoItemModel>>> rcmdVideoList({
    required int ps,
    required int freshIdx,
  }) async {
    final res = await Request().get(
      Api.recommendListWeb,
      queryParameters: await WbiSign.makSign({
        'version': 1,
        'feed_version': 'V8',
        'homepage_ver': 1,
        'ps': ps,
        'fresh_idx': freshIdx,
        'brush': freshIdx,
        'fresh_type': 4,
      }),
    );
    if (res.data['code'] == 0) {
      List<RecVideoItemModel> list = <RecVideoItemModel>[];
      for (final i in res.data['data']['item']) {
        //过滤掉live与ad，以及拉黑用户
        if (i['goto'] == 'av' &&
            (i['owner'] != null &&
                !GlobalData().blackMids.contains(i['owner']['mid']))) {
          RecVideoItemModel videoItem = RecVideoItemModel.fromJson(i);
          if (!RecommendFilter.filter(videoItem)) {
            list.add(videoItem);
          }
        }
      }
      return Success(list);
    } else {
      return Error(res.data['message']);
    }
  }

  // 添加额外的loginState变量模拟未登录状态
  static Future<LoadingState<List<RecVideoItemAppModel>>> rcmdVideoListApp({
    required int freshIdx,
  }) async {
    final params = {
      'build': 2001100,
      'c_locale': 'zh_CN',
      'channel': 'master',
      'column': 4,
      'device': 'pad',
      'device_name': 'android',
      'device_type': 0,
      'disable_rcmd': 0,
      'flush': 5,
      'fnval': 976,
      'fnver': 0,
      'force_host': 2, //使用https
      'fourk': 1,
      'guidance': 0,
      'https_url_req': 0,
      'idx': freshIdx,
      'mobi_app': 'android_hd',
      'network': 'wifi',
      'platform': 'android',
      'player_net': 1,
      'pull': freshIdx == 0 ? 'true' : 'false',
      'qn': 32,
      'recsys_mode': 0,
      's_locale': 'zh_CN',
      'splash_id': '',
      'statistics': Constants.statistics,
      'voice_balance': 0,
    };
    final res = await Request().get(
      Api.recommendListApp,
      queryParameters: params,
      options: Options(
        headers: {
          'buvid': LoginHttp.buvid,
          'fp_local':
              '1111111111111111111111111111111111111111111111111111111111111111',
          'fp_remote':
              '1111111111111111111111111111111111111111111111111111111111111111',
          'session_id': '11111111',
          'env': 'prod',
          'app-key': 'android_hd',
          'User-Agent': Constants.userAgent,
          'x-bili-trace-id': Constants.traceId,
          'x-bili-aurora-eid': '',
          'x-bili-aurora-zone': '',
          'bili-http-engine': 'cronet',
        },
      ),
    );
    if (res.data['code'] == 0) {
      List<RecVideoItemAppModel> list = <RecVideoItemAppModel>[];
      for (final i in res.data['data']['items']) {
        // 屏蔽推广和拉黑用户
        if (i['card_goto'] != 'ad_av' &&
            i['card_goto'] != 'ad_web_s' &&
            i['ad_info'] == null &&
            (i['args'] != null &&
                !GlobalData().blackMids.contains(i['args']['up_id']))) {
          if (enableFilter &&
              i['args']?['tname'] != null &&
              zoneRegExp.hasMatch(i['args']['tname'])) {
            continue;
          }
          RecVideoItemAppModel videoItem = RecVideoItemAppModel.fromJson(i);
          if (!RecommendFilter.filter(videoItem)) {
            list.add(videoItem);
          }
        }
      }
      return Success(list);
    } else {
      return Error(res.data['message']);
    }
  }

  // 最热视频
  static Future<LoadingState<List<HotVideoItemModel>>> hotVideoList({
    required int pn,
    required int ps,
  }) async {
    final res = await Request().get(
      Api.hotList,
      queryParameters: {'pn': pn, 'ps': ps},
    );
    if (res.data['code'] == 0) {
      List<HotVideoItemModel> list = <HotVideoItemModel>[];
      for (final i in res.data['data']['list']) {
        if (!GlobalData().blackMids.contains(i['owner']['mid']) &&
            !RecommendFilter.filterTitle(i['title']) &&
            !RecommendFilter.filterLikeRatio(
              i['stat']['like'],
              i['stat']['view'],
            )) {
          if (enableFilter &&
              i['tname'] != null &&
              zoneRegExp.hasMatch(i['tname'])) {
            continue;
          }
          list.add(HotVideoItemModel.fromJson(i));
        }
      }
      return Success(list);
    } else {
      return Error(res.data['message']);
    }
  }

  // 视频流
  @pragma('vm:notify-debugger-on-exception')
  static Future<LoadingState<PlayUrlModel>> videoUrl({
    int? avid,
    String? bvid,
    required int cid,
    int? qn,
    dynamic epid,
    dynamic seasonId,
    required bool tryLook,
    required VideoType videoType,
    String? language,
  }) async {
    final params = await WbiSign.makSign({
      'avid': ?avid,
      'bvid': ?bvid,
      'ep_id': ?epid,
      'season_id': ?seasonId,
      'cid': cid,
      'qn': qn ?? 80,
      // 获取所有格式的视频
      'fnval': 4048,
      'fourk': 1,
      'fnver': 0,
      'voice_balance': 1,
      'gaia_source': 'pre-load',
      'isGaiaAvoided': true,
      'web_location': 1315873,
      // 免登录查看1080p
      if (tryLook) 'try_look': 1,
      'cur_language': ?language,
    });

    try {
      final res = await Request().get(
        videoType.api,
        queryParameters: params,
      );

      if (res.data['code'] == 0) {
        late PlayUrlModel data;
        switch (videoType) {
          case VideoType.ugc:
            data = PlayUrlModel.fromJson(res.data['data']);
            break;
          case VideoType.pugv:
            final result = res.data['data'];
            data = PlayUrlModel.fromJson(result)
              ..lastPlayTime =
                  result?['play_view_business_info']?['user_status']?['watch_progress']?['current_watch_progress'];
            break;
          case VideoType.pgc:
            final result = res.data['result'];
            data = PlayUrlModel.fromJson(result['video_info'])
              ..lastPlayTime =
                  result?['play_view_business_info']?['user_status']?['watch_progress']?['current_watch_progress'];
            break;
        }

        // Query self-hosted server for watch progress
        if (SelfRequest.token != null) {
          try {
            final progRes = await SelfRequest().get(
              Api.historyProgress,
              queryParameters: {
                if (avid != null) 'aid': avid,
                if (bvid != null) 'bvid': bvid,
              },
            );
            if (progRes.data['code'] == 0) {
              final lastPlayTime = progRes.data['data']?['last_play_time'];
              if (lastPlayTime is int && lastPlayTime > 0) {
                data.lastPlayTime = lastPlayTime;
                data.lastPlayCid =
                    progRes.data['data']?['last_play_cid'] as int?;
              }
            }
          } catch (_) {}
        }

        return Success(data);
      } else if (epid != null && videoType == VideoType.ugc) {
        return videoUrl(
          avid: avid,
          bvid: bvid,
          cid: cid,
          qn: qn,
          epid: epid,
          seasonId: seasonId,
          tryLook: tryLook,
          videoType: VideoType.pgc,
        );
      }
      return Error(_parseVideoErr(res.data['code'], res.data['message']));
    } catch (e, s) {
      return Error('$e\n\n$s');
    }
  }

  static String _parseVideoErr(int? code, String? msg) {
    return switch (code) {
      -404 => '视频不存在或已被删除',
      87008 => '当前视频可能是专属视频，可能需包月充电观看($msg})',
      _ => '错误($code): $msg',
    };
  }

  // 视频信息 标题、简介
  static Future<LoadingState<VideoDetailData>> videoIntro({
    required String bvid,
  }) async {
    final res = await Request().get(
      Api.videoIntro,
      queryParameters: {'bvid': bvid},
    );
    VideoDetailResponse data = VideoDetailResponse.fromJson(res.data);
    if (data.code == 0) {
      return Success(data.data!);
    } else {
      return Error(data.message);
    }
  }

  static Future<LoadingState<VideoRelation>> videoRelation({
    required String bvid,
  }) async {
    final res = await Request().get(
      Api.videoRelation,
      queryParameters: {
        'aid': IdUtils.bv2av(bvid),
        'bvid': bvid,
      },
    );
    if (res.data['code'] == 0) {
      return Success(VideoRelation.fromJson(res.data['data']));
    } else {
      return Error(res.data['message']);
    }
  }

  // 相关视频
  static Future<LoadingState<List<HotVideoItemModel>?>> relatedVideoList({
    required String bvid,
  }) async {
    final res = await Request().get(
      Api.relatedList,
      queryParameters: {'bvid': bvid},
    );
    if (res.data['code'] == 0) {
      final items = (res.data['data'] as List?)?.map(
        (i) => HotVideoItemModel.fromJson(i),
      );
      final list = RecommendFilter.applyFilterToRelatedVideos
          ? items?.where((i) => !RecommendFilter.filterAll(i)).toList()
          : items?.toList();
      return Success(list);
    } else {
      return Error(res.data['message']);
    }
  }

  // 获取点赞/投币/收藏状态 pgc
  static Future<LoadingState<PgcLCF>> pgcLikeCoinFav({
    required Object epId,
  }) async {
    final res = await Request().get(
      Api.pgcLikeCoinFav,
      queryParameters: {'ep_id': epId},
    );
    if (res.data['code'] == 0) {
      return Success(PgcLCF.fromJson(res.data['data']));
    } else {
      return Error(res.data['message']);
    }
  }

  // 投币
  static Future<LoadingState<Null>> coinVideo({
    required String bvid,
    required int multiply,
    int selectLike = 0,
  }) async {
    final res = await Request().post(
      Api.coinVideo,
      data: {
        'aid': IdUtils.bv2av(bvid).toString(),
        // 'bvid': bvid,
        'multiply': multiply.toString(),
        'select_like': selectLike.toString(),
        // 'csrf': Accounts.main.csrf,
      },
      options: Options(contentType: Headers.formUrlEncodedContentType),
    );
    if (res.data['code'] == 0) {
      return const Success(null);
    } else {
      return Error(res.data['message']);
    }
  }

  // 一键三连 pgc
  static Future<LoadingState<PgcTriple>> pgcTriple({
    required Object epId,
    Object? seasonId,
  }) async {
    final res = await Request().post(
      Api.pgcTriple,
      data: {
        'ep_id': epId,
        'csrf': Accounts.main.csrf,
      },
      options: Options(
        contentType: Headers.formUrlEncodedContentType,
        headers: {
          'origin': 'https://www.bilibili.com',
          'referer': 'https://www.bilibili.com/bangumi/play/ss$seasonId',
          'user-agent': UaType.pc.ua,
        },
      ),
    );
    if (res.data['code'] == 0) {
      return Success(PgcTriple.fromJson(res.data['data']));
    } else {
      return Error(res.data['message']);
    }
  }

  // 一键三连
  static Future<LoadingState<UgcTriple>> ugcTriple({
    required String bvid,
  }) async {
    final res = await Request().post(
      Api.ugcTriple,
      data: {
        'aid': IdUtils.bv2av(bvid),
        'eab_x': 2,
        'ramval': 0,
        'source': 'web_normal',
        'ga': 1,
        'csrf': Accounts.main.csrf,
        'spmid': '333.788.0.0',
        'statistics': '{"appId":100,"platform":5}',
      },
      options: Options(
        contentType: Headers.formUrlEncodedContentType,
        headers: {
          'origin': 'https://www.bilibili.com',
          'referer': 'https://www.bilibili.com/video/$bvid',
          'user-agent': UaType.pc.ua,
        },
      ),
    );
    if (res.data['code'] == 0) {
      return Success(UgcTriple.fromJson(res.data['data']));
    } else {
      return Error(res.data['message']);
    }
  }

  // （取消）点赞
  static Future<LoadingState<String>> likeVideo({
    required String bvid,
    required bool type,
  }) async {
    final res = await Request().post(
      Api.likeVideo,
      data: {
        'aid': IdUtils.bv2av(bvid).toString(),
        'like': type ? '0' : '1',
      },
      options: Options(contentType: Headers.formUrlEncodedContentType),
    );
    if (res.data['code'] == 0) {
      return Success(res.data['data']['toast']);
    } else {
      return Error(res.data['message']);
    }
  }

  // （取消）点踩
  static Future<LoadingState<Null>> dislikeVideo({
    required String bvid,
    required bool type,
  }) async {
    if (Accounts.main.accessKey.isNullOrEmpty) {
      return const Error('请退出账号后重新登录');
    }
    final res = await Request().post(
      Api.dislikeVideo,
      data: {
        'aid': IdUtils.bv2av(bvid).toString(),
        'dislike': type ? '0' : '1',
      },
      options: Options(contentType: Headers.formUrlEncodedContentType),
    );
    if (res.data is! String && res.data['code'] == 0) {
      return const Success(null);
    } else {
      return Error(res.data is String ? res.data : res.data['message']);
    }
  }

  // 推送不感兴趣反馈
  static Future<LoadingState<Null>> feedDislike({
    required String goto,
    required int id,
    int? reasonId,
    int? feedbackId,
  }) async {
    if (Accounts.get(AccountType.recommend).accessKey.isNullOrEmpty) {
      return const Error('请退出账号后重新登录');
    }
    assert((reasonId != null) ^ (feedbackId != null));
    final res = await Request().get(
      Api.feedDislike,
      queryParameters: {
        'goto': goto,
        'id': id,
        'reason_id': ?reasonId,
        'feedback_id': ?feedbackId,
        'build': 1,
        'mobi_app': 'android',
      },
    );
    if (res.data['code'] == 0) {
      return const Success(null);
    } else {
      return Error(res.data['message']);
    }
  }

  // 推送不感兴趣取消
  static Future<LoadingState<Null>> feedDislikeCancel({
    required String goto,
    required int id,
    int? reasonId,
    int? feedbackId,
  }) async {
    if (Accounts.get(AccountType.recommend).accessKey.isNullOrEmpty) {
      return const Error('请退出账号后重新登录');
    }
    final res = await Request().get(
      Api.feedDislikeCancel,
      queryParameters: {
        'goto': goto,
        'id': id,
        'reason_id': ?reasonId,
        'feedback_id': ?feedbackId,
        'build': 1,
        'mobi_app': 'android',
      },
    );
    if (res.data['code'] == 0) {
      return const Success(null);
    } else {
      return Error(res.data['message']);
    }
  }

  // 发表评论 replyAdd

  // type	num	评论区类型代码	必要	类型代码见表
  // oid	num	目标评论区id	必要
  // root	num	根评论rpid	非必要	二级评论以上使用
  // parent	num	父评论rpid	非必要	二级评论同根评论id 大于二级评论为要回复的评论id
  // message	str	发送评论内容	必要	最大1000字符
  // plat	num	发送平台标识	非必要	1：web端 2：安卓客户端  3：ios客户端  4：wp客户端
  static Future<LoadingState<Map>> replyAdd({
    required int type,
    required int oid,
    required String message,
    int? root,
    int? parent,
    List? pictures,
    bool syncToDynamic = false,
    Map<String, int>? atNameToMid,
  }) async {
    final data = {
      'type': type,
      'oid': oid,
      if (root != null && root != 0) 'root': root,
      if (parent != null && parent != 0) 'parent': parent,
      'message': message,
      if (atNameToMid?.isNotEmpty == true)
        'at_name_to_mid': jsonEncode(atNameToMid), // {"name":uid}
      if (pictures != null) 'pictures': jsonEncode(pictures),
      if (syncToDynamic) 'sync_to_dynamic': 1,
      'csrf': Accounts.main.csrf,
    };
    final res = await Request().post(
      Api.replyAdd,
      data: data,
      options: Options(contentType: Headers.formUrlEncodedContentType),
    );
    if (res.data['code'] == 0) {
      return Success(res.data['data']);
    } else {
      return Error(res.data['message']);
    }
  }

  static Future<LoadingState<Null>> replyDel({
    required int type, //replyType
    required int oid,
    required int rpid,
  }) async {
    final res = await Request().post(
      Api.replyDel,
      data: {
        'type': type, //type.index
        'oid': oid,
        'rpid': rpid,
        'csrf': Accounts.main.csrf,
      },
      options: Options(contentType: Headers.formUrlEncodedContentType),
    );
    if (res.data['code'] == 0) {
      return const Success(null);
    } else {
      return const Error('请退出账号后重新登录');
    }
  }

  // 操作用户关系
  static Future<LoadingState<Null>> relationMod({
    required int mid,
    required int act,
    required int reSrc,
    String? uname,
    String? face,
  }) async {
    final res = await SelfRequest().post(
      Api.relationMod,
      data: {
        'fid': mid,
        'act': act,
        're_src': reSrc,
        if (uname != null) 'uname': uname,
        if (face != null) 'face': face,
      },
      options: Options(
        contentType: Headers.formUrlEncodedContentType,
      ),
    );
    if (res.data['code'] == 0) {
      if (act == 5) {
        // block
        Pref.setBlackMid(mid);
      } else if (act == 6) {
        // unblock
        Pref.removeBlackMid(mid);
      }
      return const Success(null);
    } else {
      return Error(res.data['message']);
    }
  }

  static Future<void> roomEntryAction({
    required Object roomId,
  }) {
    return Request().post(
      Api.roomEntryAction,
      queryParameters: {
        'csrf': Accounts.heartbeat.csrf,
      },
      data: {
        'room_id': roomId,
        'platform': 'pc',
      },
    );
  }

  static Future<void> historyReport({
    required Object aid,
    required Object type,
  }) {
    return SelfRequest().post(
      Api.historyReport,
      data: {
        'aid': aid,
        'type': type,
      },
      options: Options(contentType: Headers.formUrlEncodedContentType),
    );
  }

  // 视频播放进度
  static Future<void> heartBeat({
    Object? aid,
    Object? bvid,
    required Object cid,
    required Object progress,
    Object? epid,
    Object? seasonId,
    Object? subType,
    required VideoType videoType,
  }) {
    final isPugv = videoType == VideoType.pugv;
    return SelfRequest().post(
      Api.heartBeat,
      data: {
        if (isPugv) 'aid': ?aid else 'bvid': ?bvid,
        'cid': cid,
        'epid': ?epid,
        'sid': ?seasonId,
        'type': videoType.type,
        'sub_type': ?subType,
        'played_time': progress,
      },
      options: Options(contentType: Headers.formUrlEncodedContentType),
    );
  }

  static Future<void> medialistHistory({
    required int desc,
    required Object oid,
    required Object upperMid,
  }) {
    return SelfRequest().post(
      Api.mediaListHistory,
      data: {
        'desc': desc,
        'oid': oid,
        'upper_mid': upperMid,
      },
      options: Options(contentType: Headers.formUrlEncodedContentType),
    );
  }

  // 添加追番
  static Future<LoadingState<String>> pgcAdd({int? seasonId}) async {
    final res = await SelfRequest().post(
      Api.pgcAdd,
      data: {
        'season_id': seasonId,
      },
      options: Options(contentType: Headers.formUrlEncodedContentType),
    );
    if (res.data['code'] == 0) {
      return Success(res.data['result']['toast']);
    } else {
      return Error(res.data['message']);
    }
  }

  // 取消追番
  static Future<LoadingState<String>> pgcDel({int? seasonId}) async {
    final res = await SelfRequest().post(
      Api.pgcDel,
      data: {
        'season_id': seasonId,
      },
      options: Options(contentType: Headers.formUrlEncodedContentType),
    );
    if (res.data['code'] == 0) {
      return Success(res.data['result']['toast']);
    } else {
      return Error(res.data['message']);
    }
  }

  static Future<LoadingState<String>> pgcUpdate({
    required String seasonId,
    required int status,
  }) async {
    final res = await SelfRequest().post(
      Api.pgcUpdate,
      data: {
        'season_id': seasonId,
        'status': status,
      },
      options: Options(
        contentType: Headers.formUrlEncodedContentType,
      ),
    );
    if (res.data['code'] == 0) {
      return Success(res.data['result']['toast']);
    } else {
      return Error(res.data['message']);
    }
  }

  // 查看视频同时在看人数
  static Future<LoadingState<String>> onlineTotal({
    int? aid,
    String? bvid,
    required int cid,
  }) async {
    assert(aid != null || bvid != null);
    final res = await Request().get(
      Api.onlineTotal,
      queryParameters: {
        'aid': aid,
        'bvid': bvid,
        'cid': cid,
      },
    );
    if (res.data['code'] == 0) {
      return Success(res.data['data']['total']);
    } else {
      return Error(res.data['message']);
    }
  }

  static Future<LoadingState<AiConclusionData>> aiConclusion({
    required String bvid,
    required int cid,
    int? upMid,
  }) async {
    final params = await WbiSign.makSign({
      'bvid': bvid,
      'cid': cid,
      'up_mid': ?upMid,
    });
    final res = await Request().get(Api.aiConclusion, queryParameters: params);
    final int? code = res.data['code'];
    if (code == 0) {
      final int? dataCode = res.data['data']?['code'];
      if (dataCode == 0) {
        return Success(AiConclusionData.fromJson(res.data['data']));
      } else {
        return Error(null, code: dataCode);
      }
    } else {
      return Error(res.data['message']);
    }
  }

  static Future<LoadingState<PlayInfoData>> playInfo({
    String? aid,
    String? bvid,
    required int cid,
    dynamic seasonId,
    dynamic epId,
  }) async {
    assert(aid != null || bvid != null);
    final res = await Request().get(
      Api.playInfo,
      queryParameters: await WbiSign.makSign({
        'aid': ?aid,
        'bvid': ?bvid,
        'cid': cid,
        'season_id': ?seasonId,
        'ep_id': ?epId,
      }),
    );
    if (res.data['code'] == 0) {
      return Success(PlayInfoData.fromJson(res.data['data']));
    } else {
      return Error(res.data['message']);
    }
  }

  static String _subtitleTimecode(num seconds) {
    int h = seconds ~/ 3600;
    seconds %= 3600;
    int m = seconds ~/ 60;
    seconds %= 60;
    String sms = seconds.toStringAsFixed(3).padLeft(6, '0');
    return h == 0
        ? "${m.toString().padLeft(2, '0')}:$sms"
        : "${h.toString().padLeft(2, '0')}:${m.toString().padLeft(2, '0')}:$sms";
  }

  static num? _subtitleField(Map<String, dynamic> item, String key) {
    final value = item[key];
    if (value is num) {
      return value;
    }
    if (value is String) {
      return num.tryParse(value);
    }
    return null;
  }

  static String processList(List list) {
    final subtitles = <String>[];
    for (int i = 0; i < list.length; i++) {
      final rawItem = list[i];
      if (rawItem is! Map) {
        continue;
      }
      final item = Map<String, dynamic>.from(rawItem);
      final startSeconds = _subtitleField(item, 'from');
      final endSeconds = _subtitleField(item, 'to');
      final startMillis = _subtitleField(item, 'start_time');
      final endMillis = _subtitleField(item, 'end_time');
      final start =
          startSeconds ?? (startMillis == null ? null : startMillis / 1000);
      final end = endSeconds ?? (endMillis == null ? null : endMillis / 1000);
      final content = (item['content'] ?? item['text'] ?? '').toString().trim();
      if (start == null || end == null || content.isEmpty) {
        continue;
      }
      subtitles.add(
        '${item['sid'] ?? i + 1}\n${_subtitleTimecode(start)} --> ${_subtitleTimecode(end)}\n$content',
      );
    }
    final sb = StringBuffer('WEBVTT\n\n')..writeAll(subtitles, '\n\n');
    return sb.toString();
  }

  static Future<String?> vttSubtitles(String subtitleUrl) async {
    final res = await Request().get("https:$subtitleUrl");
    if (res.data?['body'] case List list) {
      return compute<List, String>(processList, list);
    }
    return null;
  }

  static Future<String> saucSubtitles(String audioUrl) {
    return saucSubtitlesProgressive(audioUrl);
  }

  static Future<String> saucSubtitlesProgressive(
    String audioUrl, {
    CancelToken? cancelToken,
    FutureOr<void> Function(String vtt, int completedChunks, int totalChunks)?
    onProgress,
  }) async {
    final audioUri = Uri.parse(audioUrl.http2https);
    final filename = audioUri.pathSegments.lastWhere(
      (segment) => segment.isNotEmpty,
      orElse: () => 'audio.m4a',
    );
    final audioBytes = await _downloadSubtitleAudioBytes(audioUri);

    try {
      final res = await _saucDio.post<ResponseBody>(
        '/transcribe',
        queryParameters: const {
          'progressive': '1',
          'chunk_ms': '300000',
        },
        data: FormData.fromMap({
          'file': MultipartFile.fromBytes(audioBytes, filename: filename),
        }),
        options: Options(responseType: ResponseType.stream),
        cancelToken: cancelToken,
      );
      final body = res.data;
      if (body == null) {
        throw Exception('转写服务返回为空');
      }

      final utterances = <Map<String, dynamic>>[];
      String? latestVtt;
      int totalChunks = 0;
      int completedChunks = 0;

      await for (final line
          in body.stream
              .cast<List<int>>()
              .transform(utf8.decoder)
              .transform(const LineSplitter())) {
        final text = line.trim();
        if (text.isEmpty) {
          continue;
        }
        final decoded = jsonDecode(text);
        if (decoded is! Map) {
          continue;
        }
        final data = Map<String, dynamic>.from(decoded);
        switch (data['type']) {
          case 'ready':
            if (data['total_chunks'] case final num total) {
              totalChunks = total.toInt();
            }
            break;
          case 'chunk':
            if (data['total_chunks'] case final num total) {
              totalChunks = total.toInt();
            }
            if (data['chunk_index'] case final num chunkIndex) {
              completedChunks = chunkIndex.toInt();
            }
            if (data['utterances'] case List list when list.isNotEmpty) {
              utterances.addAll(
                list.whereType<Map>().map(Map<String, dynamic>.from),
              );
              latestVtt = await compute<List, String>(
                processList,
                List<dynamic>.from(utterances),
              );
              if (latestVtt.trim().isNotEmpty && onProgress != null) {
                await onProgress(latestVtt, completedChunks, totalChunks);
              }
            }
            break;
          case 'done':
            break;
          case 'error':
            final error = (data['error'] ?? '转写服务失败').toString();
            throw Exception(error);
        }
      }

      if (latestVtt != null && latestVtt.trim().isNotEmpty) {
        return latestVtt;
      }
      throw Exception('转写结果缺少可用字幕');
    } on DioException catch (e) {
      if (CancelToken.isCancel(e)) {
        rethrow;
      }
      final data = e.response?.data;
      final message = data is Map
          ? (data['error'] ?? data['message'] ?? e.message)
          : e.message;
      throw Exception((message ?? '请求 sauc_go 失败').toString());
    }
  }

  static Future<Uint8List> _downloadSubtitleAudioBytes(Uri audioUri) async {
    final attempts = <({String name, Map<String, String> headers})>[
      (
        name: 'player-headers',
        headers: _subtitleAudioHeaders(),
      ),
    ];

    final videoCookie = _accountCookieHeader(Accounts.get(AccountType.video));
    if (videoCookie != null) {
      attempts.add((
        name: 'video-account-cookie',
        headers: _subtitleAudioHeaders(cookie: videoCookie),
      ));
    }

    final mainCookie = _accountCookieHeader(Accounts.main);
    if (mainCookie != null && mainCookie != videoCookie) {
      attempts.add((
        name: 'main-account-cookie',
        headers: _subtitleAudioHeaders(cookie: mainCookie),
      ));
    }

    final failures = <String>[];
    for (final attempt in attempts) {
      final result = await _downloadSubtitleAudioBytesOnce(
        audioUri,
        headers: attempt.headers,
      );
      final mediaKind = _detectAudioContainer(result.bytes);
      if (_isUsableSubtitleAudioPayload(result.bytes, result.contentType)) {
        return result.bytes;
      }
      failures.add(
        '${attempt.name}: '
        'status=${result.statusCode}, '
        'type=${result.contentType ?? "-"}, '
        'bytes=${result.bytes.length}, '
        'magic=${mediaKind ?? "-"}, '
        'preview=${_payloadPreview(result.bytes)}',
      );
    }

    throw Exception('下载音频失败: ${failures.join(' | ')}');
  }

  static Map<String, String> _subtitleAudioHeaders({String? cookie}) => {
    'accept-encoding': 'identity',
    'user-agent': UaType.pc.ua,
    'referer': HttpString.baseUrl,
    if (cookie != null && cookie.isNotEmpty) HttpHeaders.cookieHeader: cookie,
  };

  static String? _accountCookieHeader(Account account) {
    final cookies = account.cookieJar.toList();
    if (cookies.isEmpty) {
      return null;
    }
    return cookies.map((cookie) => '${cookie.name}=${cookie.value}').join('; ');
  }

  static Future<
    ({
      Uint8List bytes,
      String? contentType,
      int statusCode,
    })
  >
  _downloadSubtitleAudioBytesOnce(
    Uri audioUri, {
    required Map<String, String> headers,
  }) async {
    final client = HttpClient()
      ..connectionTimeout = const Duration(seconds: 15)
      ..idleTimeout = const Duration(seconds: 15)
      ..autoUncompress = false;
    if (Pref.enableSystemProxy) {
      final proxyPort = int.tryParse(Pref.systemProxyPort);
      if (proxyPort != null && Pref.systemProxyHost.isNotEmpty) {
        client.findProxy = (_) => 'PROXY ${Pref.systemProxyHost}:$proxyPort';
      }
    }
    if (Pref.badCertificateCallback) {
      client.badCertificateCallback = (cert, host, port) => true;
    }

    try {
      final request = await client.getUrl(audioUri)
        ..followRedirects = true
        ..maxRedirects = 5;
      for (final entry in headers.entries) {
        request.headers.set(entry.key, entry.value);
      }
      request.headers.set(HttpHeaders.rangeHeader, 'bytes=0-');
      final response = await request.close();
      final builder = BytesBuilder(copy: false);
      await for (final chunk in response) {
        builder.add(chunk);
      }
      return (
        bytes: builder.takeBytes(),
        contentType: response.headers.value(HttpHeaders.contentTypeHeader),
        statusCode: response.statusCode,
      );
    } finally {
      client.close(force: true);
    }
  }

  static bool _isUsableSubtitleAudioPayload(
    Uint8List bytes,
    String? contentType,
  ) {
    if (bytes.isEmpty) {
      return false;
    }

    final normalizedContentType = contentType?.toLowerCase();
    if (normalizedContentType != null) {
      if (normalizedContentType.startsWith('audio/') ||
          normalizedContentType.startsWith('video/')) {
        return true;
      }
      if (normalizedContentType.startsWith('application/octet-stream') &&
          !_looksLikeTextPayload(bytes)) {
        return true;
      }
    }

    if (_detectAudioContainer(bytes) != null) {
      return true;
    }

    if (bytes.length < 1024) {
      return false;
    }

    return !_looksLikeTextPayload(bytes);
  }

  static String? _detectAudioContainer(Uint8List bytes) {
    if (bytes.length >= 12 &&
        bytes[0] == 0x52 &&
        bytes[1] == 0x49 &&
        bytes[2] == 0x46 &&
        bytes[3] == 0x46 &&
        bytes[8] == 0x57 &&
        bytes[9] == 0x41 &&
        bytes[10] == 0x56 &&
        bytes[11] == 0x45) {
      return 'wav';
    }
    if (bytes.length >= 8 &&
        bytes[4] == 0x66 &&
        bytes[5] == 0x74 &&
        bytes[6] == 0x79 &&
        bytes[7] == 0x70) {
      return 'mp4';
    }
    if (bytes.length >= 4 &&
        bytes[0] == 0x66 &&
        bytes[1] == 0x4C &&
        bytes[2] == 0x61 &&
        bytes[3] == 0x43) {
      return 'flac';
    }
    if (bytes.length >= 4 &&
        bytes[0] == 0x1A &&
        bytes[1] == 0x45 &&
        bytes[2] == 0xDF &&
        bytes[3] == 0xA3) {
      return 'webm';
    }
    if (bytes.length >= 3 &&
        bytes[0] == 0x49 &&
        bytes[1] == 0x44 &&
        bytes[2] == 0x33) {
      return 'mp3';
    }
    if (bytes.length >= 2 && bytes[0] == 0xFF && (bytes[1] & 0xE0) == 0xE0) {
      return 'aac-or-mp3';
    }
    return null;
  }

  static bool _looksLikeTextPayload(Uint8List bytes) {
    final preview = _payloadPreview(bytes);
    if (preview.isEmpty) {
      return false;
    }
    final lower = preview.toLowerCase();
    return lower.startsWith('<!doctype') ||
        lower.startsWith('<html') ||
        lower.startsWith('{') ||
        lower.startsWith('[') ||
        lower.contains('"code"') ||
        lower.contains('"message"') ||
        lower.contains('<body') ||
        lower.contains('access denied') ||
        lower.contains('forbidden');
  }

  static String _payloadPreview(Uint8List bytes) {
    if (bytes.isEmpty) {
      return '-';
    }
    final end = bytes.length < 120 ? bytes.length : 120;
    final preview = utf8.decode(
      bytes.sublist(0, end),
      allowMalformed: true,
    );
    return preview.replaceAll(RegExp(r'\s+'), ' ').trim();
  }

  static bool _canAddRank(Map i) {
    if (!GlobalData().blackMids.contains(i['owner']['mid']) &&
        !RecommendFilter.filterTitle(i['title']) &&
        !RecommendFilter.filterLikeRatio(
          i['stat']['like'],
          i['stat']['view'],
        )) {
      if (enableFilter &&
          i['tname'] != null &&
          zoneRegExp.hasMatch(i['tname'])) {
        return false;
      }
      return true;
    }
    return false;
  }

  // 视频排行
  static Future<LoadingState<List<HotVideoItemModel>>> getRankVideoList(
    int rid,
  ) async {
    final res = await Request().get(
      Api.getRankApi,
      queryParameters: await WbiSign.makSign({
        'rid': rid,
        'type': 'all',
      }),
    );
    if (res.data['code'] == 0) {
      List<HotVideoItemModel> list = <HotVideoItemModel>[];
      for (final i in res.data['data']['list']) {
        if (_canAddRank(i)) {
          list.add(HotVideoItemModel.fromJson(i));
          // final List? others = i['others'];
          // if (others != null && others.isNotEmpty) {
          //   for (final j in others) {
          //     if (_canAddRank(j)) {
          //       list.add(HotVideoItemModel.fromJson(j));
          //     }
          //   }
          // }
        }
      }
      return Success(list);
    } else {
      return Error(res.data['message']);
    }
  }

  // pgc 排行
  static Future<LoadingState<List<PgcRankItemModel>?>> pgcRankList({
    int day = 3,
    required int seasonType,
  }) async {
    final res = await Request().get(
      Api.pgcRank,
      queryParameters: await WbiSign.makSign({
        'day': day,
        'season_type': seasonType,
      }),
    );
    if (res.data['code'] == 0) {
      return Success(
        (res.data['result']?['list'] as List?)
            ?.map((e) => PgcRankItemModel.fromJson(e))
            .toList(),
      );
    } else {
      return Error(res.data['message']);
    }
  }

  // pgc season 排行
  static Future<LoadingState<List<PgcRankItemModel>?>> pgcSeasonRankList({
    int day = 3,
    required int seasonType,
  }) async {
    final res = await Request().get(
      Api.pgcSeasonRank,
      queryParameters: await WbiSign.makSign({
        'day': day,
        'season_type': seasonType,
      }),
    );
    if (res.data['code'] == 0) {
      return Success(
        (res.data['data']?['list'] as List?)
            ?.map((e) => PgcRankItemModel.fromJson(e))
            .toList(),
      );
    } else {
      return Error(res.data['message']);
    }
  }

  static Future<LoadingState<VideoNoteData>> getVideoNoteList({
    dynamic oid,
    dynamic uperMid,
    required int page,
  }) async {
    final res = await Request().get(
      Api.archiveNoteList,
      queryParameters: {
        'csrf': Accounts.main.csrf,
        'oid': oid,
        'oid_type': 0,
        'pn': page,
        'ps': 10,
        'uper_mid': ?uperMid,
      },
    );
    if (res.data['code'] == 0) {
      return Success(VideoNoteData.fromJson(res.data['data']));
    } else {
      return Error(res.data['message']);
    }
  }

  static Future<LoadingState<List<PopularSeriesListItem>?>>
  popularSeriesList() async {
    final res = await Request().get(
      Api.popularSeriesList,
      queryParameters: await WbiSign.makSign({
        'web_location': 333.934,
      }),
    );
    if (res.data['code'] == 0) {
      return Success(
        (res.data['data']?['list'] as List<dynamic>?)
            ?.map(
              (e) => PopularSeriesListItem.fromJson(e as Map<String, dynamic>),
            )
            .toList(),
      );
    } else {
      return Error(res.data['message']);
    }
  }

  static Future<LoadingState<PopularSeriesOneData>> popularSeriesOne({
    required int number,
  }) async {
    final res = await Request().get(
      Api.popularSeriesOne,
      queryParameters: await WbiSign.makSign({
        'number': number,
        'web_location': 333.934,
      }),
    );
    if (res.data['code'] == 0) {
      return Success(PopularSeriesOneData.fromJson(res.data['data']));
    } else {
      return Error(res.data['message']);
    }
  }

  static Future<LoadingState<PopularPreciousData>> popularPrecious({
    required int page,
  }) async {
    final res = await Request().get(
      Api.popularPrecious,
      queryParameters: await WbiSign.makSign({
        'page_size': 100,
        'page': page,
        'web_location': 333.934,
      }),
    );
    if (res.data['code'] == 0) {
      return Success(PopularPreciousData.fromJson(res.data['data']));
    } else {
      return Error(res.data['message']);
    }
  }

  static Future<LoadingState<PlayUrlModel>> tvPlayUrl({
    required int cid,
    required int objectId, // aid, epid
    required int playurlType, // ugc 1, pgc 2
    int? qn,
  }) async {
    final accessKey = Accounts.get(AccountType.video).accessKey;
    final params = {
      'access_key': ?accessKey,
      'actionKey': 'appkey',
      'cid': cid,
      'fourk': 1,
      'is_proj': 1,
      'mobile_access_key': ?accessKey,
      'object_id': objectId,
      'mobi_app': 'android',
      'platform': 'android',
      'playurl_type': playurlType,
      'protocol': 0,
      'qn': qn ?? 80,
    };
    AppSign.appSign(params);
    final res = await Request().get(
      Api.tvPlayUrl,
      queryParameters: params,
    );
    if (res.data['code'] == 0) {
      return Success(PlayUrlModel.fromJson(res.data['data']));
    } else {
      return Error(res.data['message']);
    }
  }
}
