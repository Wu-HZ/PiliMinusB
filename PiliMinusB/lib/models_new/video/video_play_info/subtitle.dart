class Subtitle {
  late String lan;
  String? lanDoc;
  String? subtitleUrl;
  String? subtitleUrlV2;
  bool isAi = false;
  bool isLocal = false;
  String? transcriptionAudioUrl;

  Subtitle({
    required this.lan,
    this.lanDoc,
    this.subtitleUrl,
    this.subtitleUrlV2,
    this.isAi = false,
    this.isLocal = false,
    this.transcriptionAudioUrl,
  });

  Subtitle.fromJson(Map<String, dynamic> json) {
    lan = json["lan"];
    isAi = json["type"] == 1;
    lanDoc = '${json["lan_doc"]}${isAi ? '（AI）' : ''}';
    subtitleUrl = json["subtitle_url"];
    subtitleUrlV2 = json["subtitle_url_v2"];
  }
}
