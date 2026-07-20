from setuptools import find_packages, setup

setup(
    name="keyauth-py",
    version="0.3.6",
    description="KeyAuth SaaS Python SDK - 多租户卡密验证客户端",
    long_description=open("README.md", encoding="utf-8").read(),
    long_description_content_type="text/markdown",
    author="KeyAuth SaaS",
    license="Proprietary",
    packages=find_packages(),
    install_requires=["requests>=2.20"],
    python_requires=">=3.7",
    classifiers=[
        "Programming Language :: Python :: 3",
        "Programming Language :: Python :: 3 :: Only",
        "Operating System :: OS Independent",
    ],
)
