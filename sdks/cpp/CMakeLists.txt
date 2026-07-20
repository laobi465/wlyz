cmake_minimum_required(VERSION 3.15)
project(keyauth-cpp LANGUAGES CXX VERSION 0.4.0)

set(CMAKE_CXX_STANDARD 17)
set(CMAKE_CXX_STANDARD_REQUIRED ON)
set(CMAKE_CXX_EXTENSIONS OFF)

# 依赖：libcurl + OpenSSL
find_package(CURL REQUIRED)
find_package(OpenSSL REQUIRED)

# nlohmann/json：尝试系统包，否则从FetchContent拉取
find_package(nlohmann_json 3.2.0 QUIET)
if(NOT nlohmann_json_FOUND)
    message(STATUS "nlohmann_json 未找到，使用 FetchContent 拉取")
    include(FetchContent)
    FetchContent_Declare(
        nlohmann_json
        GIT_REPOSITORY https://github.com/nlohmann/json.git
        GIT_TAG v3.11.3
    )
    FetchContent_MakeAvailable(nlohmann_json)
endif()

add_library(keyauth STATIC
    src/keyauth.cpp
)
target_include_directories(keyauth PUBLIC
    $<BUILD_INTERFACE:${CMAKE_CURRENT_SOURCE_DIR}/include>
    $<INSTALL_INTERFACE:include>
)
target_link_libraries(keyauth PUBLIC
    CURL::libcurl
    OpenSSL::Crypto
    OpenSSL::SSL
    nlohmann_json::nlohmann_json
)

# 示例（可选）
option(KEYAUTH_BUILD_EXAMPLES "Build examples" ON)
if(KEYAUTH_BUILD_EXAMPLES)
    add_executable(keyauth_example examples/example.cpp)
    target_link_libraries(keyauth_example PRIVATE keyauth)
endif()

# 安装规则
include(GNUInstallDirs)
install(TARGETS keyauth
    ARCHIVE DESTINATION ${CMAKE_INSTALL_LIBDIR}
    LIBRARY DESTINATION ${CMAKE_INSTALL_LIBDIR}
)
install(DIRECTORY include/ DESTINATION ${CMAKE_INSTALL_INCLUDEDIR})
