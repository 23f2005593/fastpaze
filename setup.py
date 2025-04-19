from setuptools import setup, find_packages

setup(
    name="fastpaze",
    version="0.1.0",
    packages=find_packages(),
    py_modules=["fastpaze"],
    install_requires=[],
    author="FastPaze Team",
    author_email="info@fastpaze.example.com",
    description="A high-performance web framework combining Go and Python",
    long_description=open("README.md").read(),
    long_description_content_type="text/markdown",
    url="https://github.com/fastpaze/fastpaze",
    classifiers=[
        "Development Status :: 3 - Alpha",
        "Intended Audience :: Developers",
        "License :: OSI Approved :: MIT License",
        "Programming Language :: Python :: 3",
        "Programming Language :: Python :: 3.7",
        "Programming Language :: Python :: 3.8",
        "Programming Language :: Python :: 3.9",
        "Programming Language :: Python :: 3.10",
        "Programming Language :: Go",
    ],
    python_requires=">=3.7",
)