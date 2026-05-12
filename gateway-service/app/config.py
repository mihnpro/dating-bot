from pydantic_settings import BaseSettings, SettingsConfigDict


class Settings(BaseSettings):
    model_config = SettingsConfigDict(
        env_file=".env",
        env_file_encoding="utf-8",
        extra="ignore",
    )

    bot_token: str
    user_profile_service_url: str = "http://user-profile-service:8080"
    matching_service_url: str = "http://matching-service:8081"
    recommendation_service_url: str = "http://recommendation-service:8082"
    media_service_url: str = "http://media-service:8083"
    minio_internal_url: str = "http://minio:9000"
    chat_service_url: str = "http://chat-service:8083"
    chat_frontend_url: str = "http://localhost:3001"
    service_name: str = "gateway-service"
    internal_http_port: int = 8086


settings = Settings()
