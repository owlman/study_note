# 《Node.js 后端全程实战》自序

> ![全栈系列作品](https://img2023.cnblogs.com/blog/691082/202305/691082-20230529101933260-926062938.png)
>
> - 《JavaScript全栈开发》：https://book.douban.com/subject/35493728/
> - 《Vue.js全平台前端实战》：https://book.douban.com/subject/35886403/
> - 《Node.js后端全程实战》：https://book.douban.com/subject/36374893/

《Node.js 后端全程实战》这本书是本人“全栈三部曲”系列的收官之作，整个系列的创作过程起源于我某年某月某日在 Facebook 上看到的一张名为“如何成为全栈工程师”的图，图中堆叠着一摞足足半人多高的书籍，其中除了最基本的、与 HTML、CSS 相关的书籍之外，还有介绍浏览器端编程的 JavaScript 语言及其框架的书若干本，在服务器端使用的语言（例如 Java、C# 等）及其框架的书又是若干本，最后再加上关于 MySQL、SQLite3 这类数据库的以及关于 Apache 服务器的书，一共十几本，颇为壮观。总而言之，那张图无看起来无论如何都不想像是要鼓励初学者的样子。于是，我最初的设想是：尝试用一本书、一门编程语言介绍那张图中提及的主要技术知识。

然而，早在这个系列的第一本书 —— 《JavaScript 全栈开发》的审阅阶段，就不断地听到有读者反馈说：如果只使用 DOM 和 BOM 接口来编写客户端应用，或者只使用 Node.js 运行平台的原生接口编写服务端应用，那么对于大多数人来说，这都将是一个编码量巨大，调试和维护非常繁复的工作。诚然，《JavaScript 全栈开发》作为本系列作品的基础篇，它更倾向于为读者介绍 JavaScript 这门编程语言本身和客户端/服务端应用程序架构的理论基础，其中所演示的项目更多是属于我们在实验环境中所进行的各种学习和研究活动。

在现实的生产环境中，开发者们大多数时候是使用应用程序框架和专业的运维工具来应对具体项目的开发工作的。基本上，除了编程语言的基本语法之外，开发者的设计开发能力很大程度上取决于如何根据自己面对的问题找到适用的框架，并在合理的时间内掌握该框架的使用方法，并用它快速地构建自己的项目。因此在基础篇之后，我们将致力于利用具体的项目实践来向读者介绍如何构建这种“在做中学，在学中做”的能力。当然了，这些框架和工具往往都是存在适用领域边界的。换而言之，我们在从事客户端应用的开发工作时，需要使用的是适用于该领域的框架及相关的项目构建工具，而在从事服务端应用的开发与维护工作时则要使用服务端领域的框架与相关的运维工具，它们各自可能都需要用一本书的篇幅来介绍。

因此，作为《JavaScript 全栈开发》在客户端开发方向上的补充，我在去年出版了《Vue.js 全平台前端实战》一书。在该书中，我们以 Vue.js 框架及其相关工具为例为读者介绍了客户端开发工作的相关实践。而各位手中的这本书则是其在另一方向上的补充，我们将以基于 Express.js 框架及其相关的工具为例介绍 Node.js 应用程序在服务端的开发与维护。

## 0.1 本书简介

简而言之，这本书将致力于探讨在服务端领域中如何以基于 Node.js 运行平台的 Express.js 框架为中心，并搭配 Docker、Kubernetes 等服务端运维工具为读者介绍服务端应用的开发与维护工作。我们计划从 Express.js 框架的基本使用开始，循序渐进，层层深入地介绍 HTTP 服务的创建与开发、RESTful API 的设计与实现、数据库接口设计与实现、以及服务端应用的部署与维护。在这过程中，我将会在书中提供大量可读性高，可被验证的代码示例，以帮助读者理解书中所介绍的技术概念、编程思想与程序设计理念。

本书的主体将由两部分组成，第一部分介绍的是 Express.js 框架的基本使用方法，这里将用 4 章的篇幅介绍 Express.js框架本身的设计理念、核心组件、中间件机制以及项目组织方式等议题。在这部分中，我们将会具体介绍如何利用Express.js 框架创建一个基于 RESTful API 规范的服务端应用。第二部分将会介绍服务端应用的部署与运维工作。在这部分中，我们也将用 4 章的篇幅具体介绍如何使用 Docker、Kubernetes 等服务端运维工具来实现对本书第一部分开发的应用程序的自动化部署与维护。下面是本书各章的内容简介：

- **第1章：服务端开发环境**：在正式开始服务端开发的议题之前，我们会先用一章的篇幅带领读者进行一些必要的准备工作，目的是配备好后续章节中要使用的服务端环境以及相关的开发工具，并对相关的应用程序设计理念做一个概念性的介绍，帮助读者以最好的状态进入到后续的项目实践中去。

- **第2章：服务端开发方案**：在这一章中，我们首先会对 Express.js 框架做一个简单的概况介绍，目的是让读者了解这一服务端框架的核心特性及其所能带来的开发优势。然后，我们将分别演示使用 Express.js 框架实现服务端业务逻辑的两种不同方案，并根据“线上简历”这项应用的具体需求对该项目进行初始化配置和结构安排，目的是借助这一过程让读者了解基于 Express.js 框架来创建项目的基本步骤，以及这些步骤背后所反映的设计思路。

- **第3章：数据库接口设计**：在这一章中，我们将介绍数据库在服务端开发工作中所扮演的角色，以及它们在 Express.js 框架中的使用方式，在介绍过程中，我们会分别基于关系型数据库与非关系型数据库的特点来探讨数据库的接口设计，并以 MySQL、MongoDB 这两种不同类型的数据库为例来演示如何在服务端项目中设计并实现访问这些数据库的 API。

- **第4章：服务端接口实现**：在这一章中，我们会继续介绍如何使用这些 API 来实现应用程序的服务端业务逻辑，并演示如何根据 REST 设计规范来实现一个基于 C/S 架构的应用程序。在这过程中，我们将会尝试在 Express.js 项目中引入一个基于 Vue.js 框架实现的客户端。

- **第5章：非容器化部署应用**：在这一章中，我们将会具体演示如何将之前开发的“线上简历”应用程序部署到真正的服务器环境中，并以传统的、非容器化的方式对它进行维护。在这一过程中，我们将会依次为读者介绍服务端运维工作的主要内容、基本流程、所要使用的工具以及这些工具的具体使用方法。

- **第6章：应用程序的容器化**：在这一章中，我们将为读者推荐 DevOps 工作理念以及该理念所主张的容器化运维方式，目的是解决采用传统方式来进行运维工作时所要面临的麻烦。毕竟，这些麻烦不仅会给运维工作带来高昂的成本，也会因思考角度上的完全不同而在运维与开发这两项工作之间产生一些难以调和的矛盾。

- **第7章：自动化部署与维护（上）**：在这一章中，我们将致力于介绍服务端运维工作中的最后一项任务，即监控服务端应用在服务器上的运行状态，并对其进行日常维护。本章内容将涉及采用微服务架构的必要性及其容器化实现方式、Docker Compose 的安装方法以及该工具的基本使用流程，以及如何在单服务器环境中实现应用程序的自动化部署与维护。

- **第8章：自动化部署与维护（下）**：在这一章中，我们将继续介绍如何在服务器集群环境中实现应用程序的自动化部署与维护。在这一介绍过程中，我们将会带读者具体了解 Kubernetes 这一更为强大的容器编排工具，并学习其基本使用方法。

- **附录A：Git 简易教程**：版本控制系统是一种在时间维度上维护计算机程序项目的软件系统，它的功能就是方便开发者们记录并管理自己在某个特定时间节点上编写的代码，以便在必要时实现一些“有后悔药吃”的效果。在这篇附录中，我们将以 Git 这个分布式版本控制系统为例来为读者介绍这类软件工具的基本使用方式。

- **附录B：使用 Vagrant 搭建 K8s 集群**：我们在使用 VMware、VirtualBox 这一类虚拟机软件创建虚拟开发环境时，往往需要经历寻找并下载操作系统的安装镜像文件，然后根据该镜像文件启动的安装向导一步一步地安装与配置操作系统，最后还需要从零开始安装开发与运维工具，整个过程会非常的费时费力。在这篇附录中，我们将为读者具体介绍 Vagrant 这个自动化虚拟机管理工具的基本使用方法，并演示如何使用它来虚拟一个由三台服务器构成的 K8s 集群。
  
## 0.2 读者须知

由于这是一本专注于介如何使用 Express.js 框架进行开发，并对开发结果开展运维工作的小书，而 Express.js 是一个基 Node.js 运行平台的服务端应用开发框架。所以在阅读这本书之前，我们会希望读者已经掌握了 JavaScript 语言、Node.js 原生接口的基本使用方法，并了解与 HTTP 协议相关的服务器概念等基础知识。如有需要，我会建议读者先去阅读一下这本书的前作——《JavaScript全栈开发》，或者其他介绍了上述基础知识的书籍。

当然了，由于 JavaScript 社区的开发框架不仅琳琅满目，选择众多，而且新陈代谢极为快速，这意味着等到这本书写完并最终出版之时，开发者们在服务端领域很有可能已经有了比 Express.js 框架更好的选择。所以基于“授之以鱼不如授之以渔”的原则，本书的真正目的是希望帮助读者掌握快速学习任意一种新框架的能力，这需要我们更深入地去理解服务端框架的设计思路，理解为什么决定开放那些接口给用户，为什么对用户隐藏那些实现，这就需要读者自己具备开发框架的能力。换句话说，虽然不必重复发明轮子，但一个优秀的工程师或设计师应该了解轮子是如何被发明的，这样才能清楚在怎么样的轮子上构建怎么样的车。

除此之外，我在这里还需要特别强调一件事：本书中所有关于“线上简历”应用的实现代码都是基于本书各章节中的代码占比及其阅读体验等众多写作因素进行了平衡考虑之后产生的最简化版本，其中省略了绝大部分与错误处理及其他辅助功能相关的代码。因此，如果想了解实际项目中某些具体问题的解决方案，还需请读者自己去查阅本书附带源码包中的项目。当然了，在我个人看来，如果想要学好并熟练掌握一个开发框架，最好的办法就是尽可能地在实践中使用它们，在实际项目需求的驱动下模仿、试错并总结使用经验。所以在这本书中，我们并不鼓励直接复制/粘贴的本书附带源码包中的演示代码，我们更期待读者“自己动手”去模仿书中提供的示例，亲手将自己想要执行的代码输入到计算机中，观察它们是如何工作的。然后，试着修改它们，并验证其结果是否符合预期。如果符合预期，就总结当下的经验，如果不符合预期，则去思考应该做哪些调整来令其符合预期。如此周而复始，才能让学习效果事半功倍。

最后，希望如《道德经》所言：一生二，二生三，三生万物，这三本书能够帮助读者构建起成为全栈工程师的坚实基础。
